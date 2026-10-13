# Linux Foundations for Containers

> **Core principle:** Every container problem is often a Linux problem wearing a container costume.
>
> A container is not a VM. It is primarily a normal Linux process/process tree running with **isolation, resource controls, and a different filesystem view**.

---

# 1. Container Mental Model

```text
                    CONTAINER
                        │
                        ▼
                ┌───────────────┐
                │  Application  │
                │    Process    │
                └───────┬───────┘
                        │
                ┌───────▼───────┐
                │ Linux Kernel  │
                └───────┬───────┘
                        │
       ┌────────────────┼────────────────┐
       │                │                │
  Namespaces         cgroups          OverlayFS
  "What I see"     "What I get"     "What files I see"
       │                │                │
 PID/NET/etc.     CPU/Memory/etc.    layered FS
```

The fundamental Linux mechanisms behind containers are:

1. Processes
2. Signals
3. Namespaces
4. cgroups
5. OverlayFS
6. File descriptors
7. Linux system calls
8. Kernel/system logging

---

# 2. Linux Processes

A process is a running instance of a program.

Example:

```bash
python app.py
```

Linux creates a process for the running Python program.

Inspect processes:

```bash
ps
ps aux
ps -ef
pstree
```

Important fields:

```text
PID  = Process ID
PPID = Parent Process ID
```

Typical process hierarchy:

```text
init / systemd
│
├── sshd
│    └── bash
│         └── python
│
└── nginx
```

Every process has a parent, except the initial process.

---

# 3. Process Lifecycle

Simplified lifecycle:

```text
              fork()
                │
                ▼
          NEW PROCESS
                │
              exec()
                │
                ▼
             RUNNING
                │
        ┌───────┴────────┐
        │                │
      exit()           killed
        │                │
        └───────┬────────┘
                ▼
             ZOMBIE
                │
          parent wait()
                │
                ▼
             REMOVED
```

## fork()

Creates a new process.

## exec()

Replaces the current process with another program.

Conceptually:

```text
bash
 │
 └── fork()
       │
       └── child
             │
             └── exec("python")
```

The child now runs Python.

## Zombie

A zombie is a process that has exited but whose parent has not yet collected its exit status using `wait()`.

## Orphan

A process whose parent exits before it does.

It gets adopted by another process, typically PID 1 in that PID namespace.

---

# 4. PID 1

PID 1 is special.

On a normal Linux host:

```text
PID 1 → systemd/init
```

Inside a container:

```text
PID 1 → often the container's main application
```

Example:

```text
Container
│
└── PID 1: application
       ├── worker
       ├── worker
       └── child process
```

PID 1 has important responsibilities, including:

* Handling signals appropriately
* Adopting orphaned processes
* Reaping child processes

Poor PID 1 behavior can lead to:

* Zombie processes
* Improper shutdown
* Signal-handling problems

This is why container entrypoints and init processes matter.

---

# 5. Signals

Signals are kernel-mediated notifications sent to processes.

Example:

```bash
kill -TERM 1234
```

Common signals:

| Signal  | Number | Purpose                        |
| ------- | -----: | ------------------------------ |
| SIGHUP  |      1 | Hangup/reload in many programs |
| SIGINT  |      2 | Interrupt                      |
| SIGQUIT |      3 | Quit                           |
| SIGKILL |      9 | Force termination              |
| SIGTERM |     15 | Graceful termination request   |
| SIGSTOP |     19 | Stop process                   |
| SIGCONT |     18 | Continue stopped process       |

Important distinction:

```text
SIGTERM
    ↓
"Please shut down gracefully."

SIGKILL
    ↓
"Stop immediately."
```

SIGTERM can be handled.

SIGKILL cannot be caught, blocked, or ignored.

---

# 6. Container Shutdown

Conceptually:

```text
docker stop
      │
      ▼
   SIGTERM
      │
      ▼
Application cleanup
      │
      ├── finish requests
      ├── close connections
      ├── flush data/logs
      └── exit
      │
      ▼
   Process exits
```

If the application does not terminate within the configured grace period:

```text
SIGKILL
   ↓
Process forcibly terminated
```

This becomes important later in Kubernetes graceful termination.

---

# 7. cgroups

Namespaces answer:

> **What can this process see?**

cgroups answer:

> **How much of the system can this process use?**

Example:

```text
Server
│
├── Container A
│     └── Application
│
└── Container B
      └── Application
```

Without resource controls:

```text
Container A
    │
    ├── 100% CPU
    ├── huge memory consumption
    └── many processes
             ↓
      Other workloads suffer
```

cgroups provide resource control.

Modern Linux commonly uses:

# cgroups v2

cgroups v2 provides a unified hierarchy.

Conceptually:

```text
/sys/fs/cgroup/
│
├── container-A/
│    ├── cpu.max
│    ├── memory.max
│    ├── pids.max
│    └── ...
│
└── container-B/
     ├── cpu.max
     ├── memory.max
     └── ...
```

Important resources:

```text
CPU
Memory
PIDs
I/O
CPU weight
```

---

# 8. CPU Limits

Example concept:

```text
cpu.max = 20000 100000
```

Approximately means:

```text
20 ms CPU time
per
100 ms period
```

≈ 20% of one CPU.

Docker can expose this as:

```bash
docker run --cpus=0.5 ...
```

Conceptual translation:

```text
Docker option
     ↓
Container runtime
     ↓
cgroup configuration
     ↓
Linux kernel
```

---

# 9. Memory Limits

Example:

```text
memory.max = 512MB
```

If the workload exceeds its memory limit, memory pressure and OOM handling can occur.

In container orchestration you may eventually see:

```text
OOMKilled
```

Typical troubleshooting chain:

```text
Application memory usage
        ↓
cgroup memory limit
        ↓
memory pressure
        ↓
OOM handling
        ↓
process killed
```

---

# 10. Linux Namespaces

Namespaces provide isolation.

The six important namespace types are:

```text
PID
Network
Mount
UTS
IPC
User
```

Mental model:

```text
Namespaces = "What can I see?"
```

---

## 10.1 PID Namespace

Controls process ID visibility.

Host:

```text
PID 1
PID 100
PID 200
PID 300
```

Container:

```text
PID 1
PID 10
PID 11
```

A process can have different PIDs depending on which PID namespace is viewing it.

---

## 10.2 Network Namespace

Provides an isolated network environment.

Can contain its own:

```text
Network interfaces
IP addresses
Routing table
Network ports
Network stack
```

Example:

```text
Host namespace

eth0
192.168.1.20
```

Container namespace:

```text
eth0
172.17.0.2
```

Container networking is built on these Linux primitives.

---

## 10.3 Mount Namespace

Controls which filesystem mounts a process sees.

Host:

```text
/
├── home
├── etc
├── var
└── opt
```

Container:

```text
/
├── app
├── bin
├── etc
├── usr
└── ...
```

The container gets a different filesystem view.

This connects directly to OverlayFS.

---

## 10.4 UTS Namespace

Controls:

```text
Hostname
Domain name
```

Therefore:

```bash
hostname
```

can show different values inside and outside a container.

---

## 10.5 IPC Namespace

IPC = Inter-Process Communication.

It isolates IPC mechanisms such as:

```text
System V IPC
POSIX message queues
```

Processes in different IPC namespaces are isolated from each other's IPC resources.

---

## 10.6 User Namespace

Provides UID/GID isolation and mapping.

Example:

```text
Inside container:

UID 0
root

       ↓ mapping

Host:

UID 100000
```

Therefore:

> Root inside a container does not necessarily have to mean root on the host.

This is important for container security.

---

# 11. Namespace Summary

Memorize:

```text
PID    → processes I see
NET    → network I see
MOUNT  → filesystem mounts I see
UTS    → hostname I see
IPC    → IPC resources I see
USER   → user/UID mapping I have
```

---

# 12. OverlayFS

Containers commonly use layered filesystems.

Instead of making a complete copy of an image for every container, layers can be combined.

Conceptually:

```text
             Container
                │
                ▼
        ┌───────────────┐
        │    Writable   │
        │     Layer     │
        └───────────────┘
                │
        ┌───────────────┐
        │ Image Layer 3 │
        ├───────────────┤
        │ Image Layer 2 │
        ├───────────────┤
        │ Image Layer 1 │
        └───────────────┘
```

Important OverlayFS terms:

```text
lowerdir
upperdir
merged
workdir
```

Conceptually:

```text
lowerdir
   │
   │ read-only
   ▼
upperdir
   │
   │ writable changes
   ▼
merged
   │
   ▼
Application's filesystem view
```

---

# 13. Copy-on-Write / Copy-Up

Suppose an image contains:

```text
/app/config.yaml
```

The container modifies it.

The original image layer remains unchanged.

The changed file can be copied into the writable layer:

```text
Image layer
    │
    │ config.yaml
    ▼
Writable layer
    │
    │ modified config.yaml
    ▼
Merged filesystem
```

This is commonly called **copy-up**.

Important concept:

> The container's writable filesystem layer is separate from the immutable image layers.

---

# 14. File Descriptors

A file descriptor (FD) is a number representing an open resource associated with a process.

Standard descriptors:

```text
0 → stdin
1 → stdout
2 → stderr
```

Additional examples:

```text
3 → file
4 → socket
5 → pipe
6 → socket
```

Inspect:

```bash
ls -l /proc/<PID>/fd
```

Example:

```text
0 -> /dev/pts/0
1 -> /dev/pts/0
2 -> /dev/pts/0
3 -> /app/config.yaml
4 -> socket:[12345]
```

Important Linux concept:

> File descriptors can represent regular files, sockets, pipes, devices, and other resources.

---

# 15. File Descriptor Limits

Check:

```bash
ulimit -n
```

or:

```bash
cat /proc/<PID>/limits
```

Example:

```text
Max open files    1024
```

Suppose an application opens hundreds of sockets:

```text
HTTP connections
      ↓
Sockets
      ↓
File descriptors
```

Eventually it may hit its FD limit.

Typical error:

```text
Too many open files
```

Kernel-level error:

```text
EMFILE
```

This is a common production problem.

---

# 16. Linux Diagnostic Tools

These tools should become part of your container troubleshooting toolkit.

---

## ps

Inspect processes:

```bash
ps aux
ps -ef
ps -o pid,ppid,stat,cmd
```

Useful for:

```text
PID
PPID
process state
CPU
memory
command
```

---

## pstree

View the process hierarchy:

```bash
pstree
pstree -p
```

Useful for understanding:

```text
parent
  └── child
       └── grandchild
```

---

## ss

Inspect networking:

```bash
ss -tulpn
```

Options:

```text
-t → TCP
-u → UDP
-l → listening
-p → process
-n → numeric
```

Examples:

```bash
ss -lntp
ss -antp
```

Use it to determine:

* Is anything listening?
* Which port?
* Which process?
* Which connections exist?

---

## lsof

List open files/resources:

```bash
lsof
```

For a process:

```bash
lsof -p <PID>
```

Useful for:

```text
files
sockets
pipes
devices
network connections
```

---

## strace

Trace system calls:

```bash
strace <command>
```

Attach to a running process:

```bash
strace -p <PID>
```

For processes/children:

```bash
strace -f <command>
```

Useful system calls include:

```text
openat()
read()
write()
close()
connect()
accept()
execve()
clone()
```

Example debugging:

```text
Application
     ↓
open("/secret/config")
     ↓
Linux kernel
     ↓
EACCES
     ↓
Permission denied
```

`strace` lets you see this interaction.

---

## dmesg

Kernel messages:

```bash
dmesg
dmesg | tail -50
```

Useful for:

```text
OOM events
kernel errors
filesystem issues
hardware events
network/kernel events
```

Example:

```text
Out of memory:
Killed process 1234
```

The application may only report a generic failure, while `dmesg` reveals the actual kernel event.

---

## journalctl

On systemd systems:

```bash
journalctl
```

Service logs:

```bash
journalctl -u docker
```

Kernel-related journal entries:

```bash
journalctl -k
```

---

# 17. /proc — Your Linux Process Window

A very important directory:

```text
/proc
```

It exposes kernel/process information.

For PID 1234:

```text
/proc/1234/
```

Useful files:

```text
/proc/1234/status
/proc/1234/cmdline
/proc/1234/limits
/proc/1234/fd/
/proc/1234/maps
```

Examples:

```bash
cat /proc/1234/status
cat /proc/1234/limits
ls -l /proc/1234/fd
```

Think of `/proc` as:

> A window into the kernel's view of a process.

---

# 18. Container Troubleshooting Method

When a containerized application fails, don't immediately blame Docker/Kubernetes.

Work from the bottom up.

## Step 1 — Is the process alive?

```bash
ps
```

Check:

```text
PID
state
CPU
memory
```

---

## Step 2 — Is it listening?

```bash
ss -lntp
```

Check:

```text
port
address
process
```

---

## Step 3 — Are connections working?

```bash
ss -antp
```

---

## Step 4 — Are file descriptors exhausted?

```bash
ls /proc/<PID>/fd | wc -l
cat /proc/<PID>/limits
```

---

## Step 5 — Is the application stuck in a syscall?

```bash
strace -p <PID>
```

---

## Step 6 — Is the kernel reporting a problem?

```bash
dmesg | tail -50
journalctl -k
```

---

## Step 7 — Is the process hitting resource limits?

Investigate:

```text
CPU
Memory
PIDs
I/O
File descriptors
```

---

## Step 8 — Is there a filesystem problem?

Investigate:

```text
Mount namespace
OverlayFS
Permissions
Read-only filesystem
Missing files
```

---

# 19. The Full Container Mental Model

Eventually, when you see:

```bash
docker run \
  --memory=512m \
  --cpus=1 \
  nginx
```

you should mentally translate it into:

```text
Docker
  │
  ▼
Container Runtime
  │
  ├── Namespaces
  │      │
  │      ├── PID
  │      ├── Network
  │      ├── Mount
  │      ├── UTS
  │      ├── IPC
  │      └── User
  │
  ├── cgroups
  │      │
  │      ├── CPU
  │      ├── Memory
  │      └── PIDs
  │
  ├── Filesystem
  │      │
  │      └── OverlayFS
  │
  └── Process
         │
         ├── PID
         ├── Signals
         └── File Descriptors
```

The container runtime configures these Linux primitives.

The Linux kernel actually enforces them.

---

# 20. The Most Important Distinction

Remember these three questions:

```text
NAMESPACES
"What can the process SEE?"

CGROUPS
"What resources can the process USE?"

OVERLAYFS
"What filesystem VIEW does the process GET?"
```

Then:

```text
PROCESS
"What is actually running?"

SIGNALS
"How do we control/terminate it?"

FILE DESCRIPTORS
"What resources/connections does it have open?"

SYSTEM CALLS
"What is the process asking the kernel to do?"
```

This mental model is more valuable than memorizing Docker commands.

---

# 21. Lab Roadmap

Before moving deeply into Docker/Kubernetes, practice these labs.

## Lab 1 — Processes

Learn:

```bash
ps
pstree
/proc/<PID>/
```

Understand:

```text
PID
PPID
fork
exec
exit
zombie
orphan
PID 1
```

---

## Lab 2 — Signals

Practice:

```bash
kill
SIGTERM
SIGKILL
SIGSTOP
SIGCONT
```

Build a small Python program that handles SIGTERM gracefully.

---

## Lab 3 — Namespaces

Explore:

```bash
lsns
unshare
nsenter
```

Observe differences in:

```text
PID
Network
Mount
Hostname
User
```

---

## Lab 4 — cgroups v2

Inspect:

```bash
/sys/fs/cgroup/
```

Experiment with:

```text
CPU limits
Memory limits
PID limits
```

---

## Lab 5 — OverlayFS

Create:

```text
lower/
upper/
work/
merged/
```

Mount OverlayFS and observe:

```text
read from lower
write to upper
merged filesystem view
copy-up
```

---

## Lab 6 — File Descriptors

Practice:

```bash
lsof
ulimit
/proc/<PID>/fd
/proc/<PID>/limits
```

Investigate:

```text
open files
sockets
pipes
FD exhaustion
```

---

## Lab 7 — System Calls

Use:

```bash
strace
```

Investigate:

```text
file not found
permission denied
network connection failure
process execution
```

---

## Lab 8 — Kernel Diagnostics

Practice:

```bash
dmesg
journalctl
```

Correlate:

```text
Application behavior
        ↓
System call
        ↓
Kernel behavior
        ↓
Kernel log
```

---

# 22. Final Mental Model

The entire subject can be reduced to:

```text
                 CONTAINER
                     │
                     ▼
               Linux Process
                     │
        ┌────────────┼────────────┐
        │            │            │
   Namespace       cgroup      Filesystem
        │            │            │
   "What I see"  "What I get"  "What I see"
        │            │            │
 PID / NET /      CPU / RAM /    OverlayFS
 MOUNT / USER     PIDs / I/O
        │            │            │
        └────────────┼────────────┘
                     │
                     ▼
                Linux Kernel
                     │
                     ▼
                Hardware
```

And when something breaks:

```text
Container problem
      ↓
Is the process alive?
      ↓
Are signals working?
      ↓
Is it listening?
      ↓
Are networking namespaces correct?
      ↓
Are file descriptors exhausted?
      ↓
Are cgroup limits being hit?
      ↓
Is the filesystem/mount correct?
      ↓
What system call is failing?
      ↓
What does the kernel say?
```

> **Goal:** Don't memorize "Docker troubleshooting commands."
> Understand what Linux is doing underneath the container, and the commands become tools for answering specific questions.
