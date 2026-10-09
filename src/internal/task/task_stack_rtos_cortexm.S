.syntax unified
.thumb

// ARM DDI 0337E sections 5.5 and 5.6 describe exception entry and return.
// https://documentation-service.arm.com/static/6036810d5319e554d4ba108e

.section .bss.tinygo_rtos_context,"aw",%nobits
.balign 4
tinygo_rtos_system_sp:
    .space 4
tinygo_rtos_current_sp:
    .space 4

.section .text.tinygo_startTask,"ax",%progbits
.global tinygo_startTask
.type tinygo_startTask,%function
.thumb_func
tinygo_startTask:
    mov r0, r5
    blx r4
    bl tinygo_task_exit
    udf #0
.size tinygo_startTask, .-tinygo_startTask

.section .text.tinygo_rtos_disablePreemption,"ax",%progbits
.global tinygo_rtos_disablePreemption
.type tinygo_rtos_disablePreemption,%function
.thumb_func
tinygo_rtos_disablePreemption:
    mrs r0, BASEPRI
    movs r1, #0xf0
    msr BASEPRI_MAX, r1
    dsb
    isb
    bx lr
.size tinygo_rtos_disablePreemption, .-tinygo_rtos_disablePreemption

.section .text.tinygo_rtos_restorePreemption,"ax",%progbits
.global tinygo_rtos_restorePreemption
.type tinygo_rtos_restorePreemption,%function
.thumb_func
tinygo_rtos_restorePreemption:
    msr BASEPRI, r0
    dsb
    isb
    bx lr
.size tinygo_rtos_restorePreemption, .-tinygo_rtos_restorePreemption

.section .text.tinygo_rtos_switchToTask,"ax",%progbits
.global tinygo_rtos_switchToTask
.type tinygo_rtos_switchToTask,%function
.thumb_func
tinygo_rtos_switchToTask:
    svc #0
    bx lr
.size tinygo_rtos_switchToTask, .-tinygo_rtos_switchToTask

.section .text.tinygo_rtos_switchToScheduler,"ax",%progbits
.global tinygo_rtos_switchToScheduler
.type tinygo_rtos_switchToScheduler,%function
.thumb_func
tinygo_rtos_switchToScheduler:
    svc #1
    bx lr
.size tinygo_rtos_switchToScheduler, .-tinygo_rtos_switchToScheduler

.section .text.tinygo_rtos_exceptions,"ax",%progbits
.global SVC_Handler
.type SVC_Handler,%function
.thumb_func
SVC_Handler:
    cpsid i
    ldr r2, =0xe000ed04
    mov.w r3, #0x08000000
    str r3, [r2]
    tst lr, #4
    bne .Lsave_task

    mrs r0, MSP
    ldr r3, [r0]
    ldr r2, =tinygo_rtos_current_sp
    str r3, [r2]
    mrs r2, BASEPRI
    stmdb r0!, {r4-r11}
    sub r0, r0, #8
    str r2, [r0]
    str lr, [r0, #4]
    ldr r2, =tinygo_rtos_system_sp
    str r0, [r2]
    msr MSP, r0
    ldr r0, [r3]
    b .Lrestore_context

.Lsave_task:
    mrs r0, PSP
    mrs r2, BASEPRI
    stmdb r0!, {r4-r11}
    sub r0, r0, #8
    str r2, [r0]
    str lr, [r0, #4]
    ldr r3, =tinygo_rtos_current_sp
    ldr r3, [r3]
    str r0, [r3]
    ldr r0, =tinygo_rtos_system_sp
    ldr r0, [r0]
    b .Lrestore_context
.size SVC_Handler, .-SVC_Handler

.global PendSV_Handler
.type PendSV_Handler,%function
.thumb_func
PendSV_Handler:
    tst lr, #4
    beq .Lignore_preemption
    cpsid i
    mrs r0, PSP
    mrs r2, BASEPRI
    stmdb r0!, {r4-r11}
    sub r0, r0, #8
    str r2, [r0]
    str lr, [r0, #4]
    ldr r3, =tinygo_rtos_current_sp
    ldr r3, [r3]
    str r0, [r3]
    push {r0, lr}
    bl tinygo_rtos_preempt
    mov r2, r0
    pop {r0, lr}
    cmp r2, #0
    beq .Lrestore_context
    ldr r0, =tinygo_rtos_system_sp
    ldr r0, [r0]
    b .Lrestore_context
.Lignore_preemption:
    bx lr
.size PendSV_Handler, .-PendSV_Handler

.Lrestore_context:
    ldmia r0!, {r2, r3}
    ldmia r0!, {r4-r11}
    mov lr, r3
    tst lr, #4
    ite ne
    msrne PSP, r0
    msreq MSP, r0
    msr BASEPRI, r2
    dsb
    isb
    cpsie i
    bx lr
    .ltorg
