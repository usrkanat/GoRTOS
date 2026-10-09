#!/usr/bin/env python3
"""Test the production Cortex-M3 handlers with explicit exception entry and return."""

import pathlib
import re
import struct
import subprocess
import tempfile
import unittest

from elftools.elf.elffile import ELFFile
from unicorn import Uc, UC_ARCH_ARM, UC_HOOK_CODE, UC_MODE_MCLASS, UC_MODE_THUMB
from unicorn import arm_const as arm


CODE = 0x10000
RAM = 0x20000000
SYSTEM_TOP = RAM + 0xF000
TASK_TOP = RAM + 0x8000
STATE = RAM + 0x100
STOP = CODE + 0xF000
REGS = [getattr(arm, f"UC_ARM_REG_R{i}") for i in range(13)]
HARDWARE_REGS = REGS[:4] + [REGS[12], arm.UC_ARM_REG_LR, arm.UC_ARM_REG_PC]


def words(values):
    return struct.pack("<" + "I" * len(values), *values)


class ContextTests(unittest.TestCase):
    @classmethod
    def setUpClass(cls):
        root = pathlib.Path(__file__).resolve().parents[1]
        source = (root / "src/internal/task/task_stack_rtos_cortexm.S").read_text()
        source = re.sub(r"^\.section .*", ".text", source, flags=re.MULTILINE)
        source += """
.thumb_func
tinygo_task_exit:
    bx lr
.thumb_func
tinygo_rtos_preempt:
    ldr r0, =test_preempt_result
    ldr r0, [r0]
    bx lr
.balign 4
test_preempt_result:
    .word 1
.ltorg
"""
        with tempfile.TemporaryDirectory(prefix="rtos-context-") as directory:
            assembly = pathlib.Path(directory) / "context.S"
            obj = pathlib.Path(directory) / "context.o"
            assembly.write_text(source)
            subprocess.run([
                "clang", "-target", "arm-none-eabi", "-mcpu=cortex-m3",
                "-mthumb", "-c", str(assembly), "-o", str(obj),
            ], check=True)
            with obj.open("rb") as stream:
                elf = ELFFile(stream)
                cls.binary = bytearray(elf.get_section_by_name(".text").data())
                symbols = elf.get_section_by_name(".symtab")
                cls.symbols = {
                    s.name: CODE + (s["st_value"] & ~1)
                    for s in symbols.iter_symbols() if s.name
                }
                relocations = elf.get_section_by_name(".rel.text")
                for relocation in relocations.iter_relocations():
                    if relocation["r_info_type"] != 2:
                        raise AssertionError("unexpected assembly relocation")
                    offset = relocation["r_offset"]
                    value = struct.unpack_from("<I", cls.binary, offset)[0]
                    symbol = symbols.get_symbol(relocation["r_info_sym"])
                    struct.pack_into("<I", cls.binary, offset, value + CODE + symbol["st_value"])

    def setUp(self):
        self.cpu = Uc(UC_ARCH_ARM, UC_MODE_THUMB | UC_MODE_MCLASS)
        self.cpu.mem_map(CODE, 0x10000)
        self.cpu.mem_map(RAM, 0x10000)
        self.cpu.mem_map(0xE000E000, 0x1000)
        self.cpu.mem_write(CODE, bytes(self.binary))
        self.cpu.reg_write(arm.UC_ARM_REG_MSP, SYSTEM_TOP)
        self.cpu.reg_write(arm.UC_ARM_REG_PSP, TASK_TOP)
        self.cpu.reg_write(arm.UC_ARM_REG_XPSR, 1 << 24)
        self.callback_count = 0
        self.returned = False
        self.cpu.hook_add(UC_HOOK_CODE, self.on_instruction)

    def read_word(self, address):
        return struct.unpack("<I", self.cpu.mem_read(address, 4))[0]

    def write_word(self, address, value):
        self.cpu.mem_write(address, words([value]))

    def on_instruction(self, cpu, address, size, data):
        if address == self.symbols["tinygo_rtos_preempt"]:
            self.assertEqual(cpu.reg_read(arm.UC_ARM_REG_MSP) % 8, 0)
            self.callback_count += 1
        if address == STOP:
            self.returned = True
            cpu.emu_stop()
            return
        lr = cpu.reg_read(arm.UC_ARM_REG_LR)
        if cpu.mem_read(address, 2) != b"\x70\x47" or lr not in (0xFFFFFFF9, 0xFFFFFFFD):
            return
        stack = arm.UC_ARM_REG_PSP if lr & 4 else arm.UC_ARM_REG_MSP
        sp = cpu.reg_read(stack)
        frame = struct.unpack("<8I", cpu.mem_read(sp, 32))
        cpu.reg_write(arm.UC_ARM_REG_XPSR, frame[7])
        cpu.reg_write(arm.UC_ARM_REG_CONTROL, 2 if lr & 4 else 0)
        cpu.reg_write(stack, sp + 32 + (4 if frame[7] & (1 << 9) else 0))
        for register, value in zip(HARDWARE_REGS, frame[:7]):
            cpu.reg_write(register, value)
        self.returned = True
        cpu.emu_stop()

    def register_values(self):
        return [self.cpu.reg_read(r) for r in REGS] + [self.cpu.reg_read(arm.UC_ARM_REG_LR)]

    def enter(self, handler, task, values, basepri=0, padding=False):
        for register, value in zip(REGS, values[:13]):
            self.cpu.reg_write(register, value)
        stack = arm.UC_ARM_REG_PSP if task else arm.UC_ARM_REG_MSP
        sp = self.cpu.reg_read(stack)
        frame = values[:4] + [values[12], values[13], STOP, (1 << 24) | (padding << 9)]
        self.cpu.mem_write(sp - 32 - (4 if padding else 0), words(frame))
        self.cpu.reg_write(stack, sp - 32 - (4 if padding else 0))
        self.cpu.reg_write(arm.UC_ARM_REG_BASEPRI, basepri)
        self.cpu.reg_write(arm.UC_ARM_REG_XPSR, (1 << 24) | (11 if handler == "SVC_Handler" else 14))
        self.cpu.reg_write(arm.UC_ARM_REG_LR, 0xFFFFFFFD if task else 0xFFFFFFF9)
        self.returned = False
        self.cpu.emu_start(self.symbols[handler] | 1, STOP, count=500)
        self.assertTrue(self.returned, "handler did not return")
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_PRIMASK), 0)

    def start_task(self):
        self.system = [0x1100 + i for i in range(14)]
        self.system[0] = STATE
        self.initial = [0x2200 + i for i in range(14)]
        frame = [0, 0xFFFFFFFD] + self.initial[4:12]
        frame += self.initial[:4] + [self.initial[12], self.initial[13], STOP, 1 << 24]
        self.cpu.mem_write(TASK_TOP - 72, words(frame))
        self.write_word(STATE, TASK_TOP - 72)
        self.enter("SVC_Handler", False, self.system)
        self.assertEqual(self.register_values(), self.initial)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_PSP), TASK_TOP)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_BASEPRI), 0)

    def test_voluntary_round_trip(self):
        self.start_task()
        task = [0x3300 + i for i in range(14)]
        self.enter("SVC_Handler", True, task, basepri=0xF0)
        self.assertEqual(self.register_values(), self.system)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_MSP), SYSTEM_TOP)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_BASEPRI), 0)
        self.assertEqual(self.read_word(STATE), TASK_TOP - 72)
        self.enter("SVC_Handler", False, self.system)
        self.assertEqual(self.register_values(), task)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_BASEPRI), 0xF0)

    def test_preemption_round_trip(self):
        self.start_task()
        task = [0x4400 + i for i in range(14)]
        self.enter("PendSV_Handler", True, task)
        self.assertEqual(self.callback_count, 1)
        self.assertEqual(self.register_values(), self.system)
        self.enter("SVC_Handler", False, self.system)
        self.assertEqual(self.register_values(), task)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_PSP), TASK_TOP)

    def test_rejected_preemption(self):
        self.start_task()
        self.write_word(self.symbols["test_preempt_result"], 0)
        task = [0x5500 + i for i in range(14)]
        self.enter("PendSV_Handler", True, task)
        self.assertEqual(self.callback_count, 1)
        self.assertEqual(self.register_values(), task)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_PSP), TASK_TOP)

    def test_scheduler_is_not_preempted(self):
        values = [0x6600 + i for i in range(14)]
        self.enter("PendSV_Handler", False, values)
        self.assertEqual(self.callback_count, 0)
        self.assertEqual(self.register_values(), values)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_MSP), SYSTEM_TOP)

    def test_hardware_alignment_padding(self):
        self.start_task()
        self.cpu.reg_write(arm.UC_ARM_REG_PSP, TASK_TOP - 4)
        task = [0x7700 + i for i in range(14)]
        self.enter("PendSV_Handler", True, task, padding=True)
        self.assertEqual(self.register_values(), self.system)
        self.enter("SVC_Handler", False, self.system)
        self.assertEqual(self.register_values(), task)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_PSP), TASK_TOP - 4)

    def call(self, symbol, argument=0):
        self.cpu.reg_write(arm.UC_ARM_REG_R0, argument)
        self.cpu.reg_write(arm.UC_ARM_REG_LR, STOP | 1)
        self.returned = False
        self.cpu.emu_start(self.symbols[symbol] | 1, STOP + 2, count=100)
        self.assertTrue(self.returned)
        return self.cpu.reg_read(arm.UC_ARM_REG_R0)

    def test_nested_preemption_masks(self):
        outer = self.call("tinygo_rtos_disablePreemption")
        self.assertEqual(outer, 0)
        inner = self.call("tinygo_rtos_disablePreemption")
        self.assertEqual(inner, 0xF0)
        self.call("tinygo_rtos_restorePreemption", inner)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_BASEPRI), 0xF0)
        self.call("tinygo_rtos_restorePreemption", outer)
        self.assertEqual(self.cpu.reg_read(arm.UC_ARM_REG_BASEPRI), 0)


if __name__ == "__main__":
    unittest.main(verbosity=2)
