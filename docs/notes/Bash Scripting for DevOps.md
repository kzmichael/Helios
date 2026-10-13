# Bash Scripting for DevOps

## From Shell Basics to Defensive, Idempotent Automation

> **Goal:** Build enough Bash understanding to write reliable DevOps automation rather than copying shell commands from documentation.
>
> **Scope:** Variables, quoting, arguments, arrays, conditions, loops, functions, exit codes, `set -euo pipefail`, traps, cleanup, pipelines, here-documents, process substitution, and idempotent scripting.

---

# 1. Why Bash Matters in DevOps

Bash sits directly on top of the Linux operating system.

A DevOps engineer frequently uses it for:

* provisioning servers
* installing software
* configuring Linux
* manipulating files
* querying processes
* automating deployments
* interacting with APIs and CLIs
* CI/CD pipelines
* Docker entrypoints
* Kubernetes troubleshooting
* cloud automation
* backup scripts
* health checks
* operational runbooks

For example:

```text
CI/CD Pipeline
      │
      ▼
 Bash Script
      │
      ├── AWS CLI
      ├── kubectl
      ├── docker
      ├── terraform
      ├── curl
      └── Linux commands
```

Bash is therefore often the **glue** between tools.

---

# 2. The Most Important Mental Model

A Bash script is not just a list of commands.

A good automation script follows this model:

```text
INPUT
  │
  ▼
VALIDATE
  │
  ▼
INSPECT CURRENT STATE
  │
  ▼
COMPARE WITH DESIRED STATE
  │
  ▼
MAKE CHANGES
  │
  ▼
VERIFY RESULT
  │
  ▼
CLEAN UP
  │
  ▼
EXIT WITH STATUS
```

This is the mindset we want for DevOps.

---

# 3. Your First Bash Script

Create:

```bash
hello.sh
```

Contents:

```bash
#!/usr/bin/env bash

echo "Hello DevOps"
```

Make it executable:

```bash
chmod +x hello.sh
```

Run:

```bash
./hello.sh
```

Output:

```text
Hello DevOps
```

---

# 4. The Shebang

The first line:

```bash
#!/usr/bin/env bash
```

is called the **shebang**.

It tells the operating system which interpreter should execute the script.

Another common form is:

```bash
#!/bin/bash
```

For portable Bash scripts, this is commonly preferred:

```bash
#!/usr/bin/env bash
```

because it asks `env` to locate Bash.

---

# 5. Bash Commands

A simple script:

```bash
#!/usr/bin/env bash

echo "Starting"

pwd

ls

echo "Finished"
```

Commands execute sequentially:

```text
echo
 ↓
pwd
 ↓
ls
 ↓
echo
```

Unless control flow or failure handling changes the execution path.

---

# 6. Variables

Variables store values.

```bash
name="Kshitiz"
environment="production"
region="ap-south-1"
```

Access them using `$`:

```bash
echo "$name"
echo "$environment"
echo "$region"
```

Output:

```text
Kshitiz
production
ap-south-1
```

---

# 7. No Spaces Around `=`

Correct:

```bash
name="Kshitiz"
```

Incorrect:

```bash
name = "Kshitiz"
```

Bash interprets commands and assignments differently.

Think:

```text
variable=value
```

not:

```text
variable = value
```

---

# 8. Variables Are Strings by Default

Consider:

```bash
count=10
```

Bash does not treat this like a strongly typed programming language would.

For arithmetic:

```bash
result=$((count + 5))
```

Then:

```bash
echo "$result"
```

Output:

```text
15
```

---

# 9. Command Substitution

One of the most useful Bash features:

```bash
hostname=$(hostname)
```

The command:

```bash
hostname
```

runs first.

Its output is captured and assigned to the variable.

Equivalent mental model:

```text
hostname command
      │
      ▼
"server01"
      │
      ▼
hostname="server01"
```

Another example:

```bash
current_date=$(date)
```

Use modern command substitution:

```bash
$(command)
```

rather than the older:

```bash
`command`
```

---

# 10. Quoting — One of the Most Important Bash Topics

Consider:

```bash
file="my report.txt"
```

Now:

```bash
cat $file
```

can effectively become:

```text
cat my report.txt
```

which Bash interprets as multiple words.

But:

```bash
cat "$file"
```

passes the entire value as one argument.

Therefore:

> **Quote variable expansions by default.**

Use:

```bash
"$variable"
```

rather than:

```bash
$variable
```

unless you deliberately want Bash's word splitting or glob expansion.

---

# 11. What Happens When You Don't Quote?

Suppose:

```bash
directory="/opt/my application"
```

Then:

```bash
cd $directory
```

can be interpreted approximately as:

```text
cd /opt/my application
```

which means two arguments:

```text
/opt/my
application
```

Instead:

```bash
cd "$directory"
```

passes one argument:

```text
/opt/my application
```

---

# 12. Word Splitting

Bash performs several forms of expansion.

One particularly important one is **word splitting**.

Example:

```bash
name="hello world"
```

Unquoted:

```bash
echo $name
```

can be split into:

```text
hello
world
```

Quoted:

```bash
echo "$name"
```

remains:

```text
hello world
```

This is one reason Bash scripts are full of quotes.

---

# 13. Globbing

Bash also expands patterns such as:

```bash
*.log
```

Suppose:

```text
app.log
error.log
access.log
```

Then:

```bash
echo *.log
```

may expand to:

```text
echo app.log error.log access.log
```

This is called **pathname expansion/globbing**.

Quoting changes this behavior:

```bash
echo "*.log"
```

prints:

```text
*.log
```

rather than expanding the wildcard.

---

# 14. Single vs Double Quotes

This distinction matters.

## Double quotes

```bash
"$name"
```

allow variable expansion.

Example:

```bash
name="Kshitiz"

echo "Hello $name"
```

Output:

```text
Hello Kshitiz
```

## Single quotes

```bash
'$name'
```

treat the content literally.

Example:

```bash
echo 'Hello $name'
```

Output:

```text
Hello $name
```

Mental model:

```text
"..." → expansion happens

'...' → literal text
```

---

# 15. Curly Braces Around Variables

Usually:

```bash
"$name"
```

is sufficient.

But sometimes you need to explicitly mark where the variable name ends.

Example:

```bash
environment="prod"

echo "${environment}_backup"
```

Without braces, Bash could interpret:

```bash
$environment_backup
```

as a completely different variable.

Use:

```bash
"${variable}_suffix"
```

when concatenating.

---

# 16. Positional Arguments

When a script is executed:

```bash
./deploy.sh production ap-south-1
```

Bash provides positional parameters.

```text
$0 → ./deploy.sh
$1 → production
$2 → ap-south-1
```

Example:

```bash
#!/usr/bin/env bash

echo "Script: $0"
echo "Environment: $1"
echo "Region: $2"
```

Output:

```text
Script: ./deploy.sh
Environment: production
Region: ap-south-1
```

---

# 17. Important Special Variables

| Variable | Meaning                                         |
| -------- | ----------------------------------------------- |
| `$0`     | Script name                                     |
| `$1`     | First argument                                  |
| `$2`     | Second argument                                 |
| `$#`     | Number of arguments                             |
| `$@`     | All positional arguments                        |
| `$?`     | Exit code of previous command                   |
| `$$`     | Current shell PID                               |
| `$!`     | PID of most recently started background process |

---

# 18. `$@` — All Arguments

Suppose:

```bash
./script.sh "hello world" "foo bar"
```

There are two arguments:

```text
Argument 1 = hello world
Argument 2 = foo bar
```

Use:

```bash
for arg in "$@"; do
    echo "$arg"
done
```

Output:

```text
hello world
foo bar
```

The quotes around:

```bash
"$@"
```

are extremely important.

---

# 19. `$*` vs `$@`

This is a classic Bash interview question.

Consider:

```bash
./script.sh "hello world" "foo bar"
```

Inside double quotes:

```bash
"$@"
```

preserves each argument separately:

```text
"hello world"
"foo bar"
```

Whereas:

```bash
"$*"
```

combines the arguments into a single word.

For most argument-forwarding situations:

```bash
"$@"
```

is what you want.

---

# 20. Arrays

Bash supports arrays.

```bash
servers=("web01" "web02" "web03")
```

Access individual elements:

```bash
echo "${servers[0]}"
```

Output:

```text
web01
```

Second:

```bash
echo "${servers[1]}"
```

---

# 21. Array Length

```bash
echo "${#servers[@]}"
```

If:

```bash
servers=("web01" "web02" "web03")
```

the result is:

```text
3
```

---

# 22. The Correct Way to Loop Through an Array

Use:

```bash
for server in "${servers[@]}"; do
    echo "$server"
done
```

This preserves each element as a separate word.

Suppose:

```bash
servers=("web 01" "web 02")
```

Using:

```bash
"${servers[@]}"
```

preserves:

```text
web 01
web 02
```

as two separate array elements.

---

# 23. `"${arr[@]}"` vs `"${arr[*]}"`

This is worth memorizing.

```bash
"${arr[@]}"
```

means:

> Expand each array element as a separate word.

```bash
"${arr[*]}"
```

inside quotes means:

> Combine all elements into one word.

For normal array iteration:

```bash
for item in "${arr[@]}"; do
    ...
done
```

is the standard pattern.

---

# 24. Conditional Statements

Basic structure:

```bash
if [[ condition ]]; then
    commands
elif [[ another_condition ]]; then
    commands
else
    commands
fi
```

Example:

```bash
environment="production"

if [[ "$environment" == "production" ]]; then
    echo "Production deployment"
else
    echo "Non-production deployment"
fi
```

---

# 25. Why `[[ ]]`?

For Bash scripts, prefer:

```bash
[[ ... ]]
```

for conditions.

It provides safer and more predictable Bash-specific behavior than the older:

```bash
[ ... ]
```

You may still see `[ ]` everywhere in older shell scripts.

---

# 26. Common Tests

### String equality

```bash
[[ "$environment" == "production" ]]
```

### String inequality

```bash
[[ "$environment" != "production" ]]
```

### String empty

```bash
[[ -z "$value" ]]
```

### String non-empty

```bash
[[ -n "$value" ]]
```

### File exists

```bash
[[ -f "$file" ]]
```

### Directory exists

```bash
[[ -d "$directory" ]]
```

### Executable exists

```bash
[[ -x "$file" ]]
```

---

# 27. Logical Operators

AND:

```bash
if [[ "$environment" == "production" && "$region" == "ap-south-1" ]]; then
    ...
fi
```

OR:

```bash
if [[ "$environment" == "production" || "$environment" == "staging" ]]; then
    ...
fi
```

NOT:

```bash
if [[ ! -f "$file" ]]; then
    ...
fi
```

---

# 28. Loops

## `for`

```bash
for server in "${servers[@]}"; do
    echo "Checking $server"
done
```

## C-style `for`

```bash
for ((i=0; i<5; i++)); do
    echo "$i"
done
```

## `while`

```bash
while [[ condition ]]; do
    ...
done
```

---

# 29. Reading a File Line by Line

A common DevOps pattern:

```bash
while IFS= read -r line; do
    echo "$line"
done < servers.txt
```

Important parts:

```text
IFS=
→ prevents unwanted trimming/splitting

read -r
→ prevents backslash interpretation

< servers.txt
→ redirects the file into the loop
```

This is safer than many simplistic:

```bash
for line in $(cat file)
```

patterns.

---

# 30. Functions

Functions allow you to organize scripts.

```bash
deploy() {
    echo "Deploying application"
}
```

Call it:

```bash
deploy
```

---

# 31. Function Arguments

```bash
deploy() {
    local environment="$1"

    echo "Deploying to $environment"
}

deploy "production"
```

Inside the function:

```text
$1 → function's first argument
```

Use:

```bash
local
```

for variables that should remain local to the function.

---

# 32. Why Functions Matter in DevOps

Instead of:

```text
100 lines of commands
```

structure the script:

```text
main
 ├── validate_inputs
 ├── check_prerequisites
 ├── prepare
 ├── deploy
 ├── verify
 └── cleanup
```

This makes operational scripts much easier to understand and troubleshoot.

---

# 33. Exit Codes

Every Unix process returns an exit status.

The convention is:

```text
0      → success
nonzero → failure
```

Example:

```bash
true
echo "$?"
```

returns:

```text
0
```

And:

```bash
false
echo "$?"
```

returns:

```text
1
```

---

# 34. Why Exit Codes Matter in DevOps

Consider a CI/CD pipeline:

```text
Build
 ↓
Test
 ↓
Deploy
```

Suppose the deployment script encounters an error but returns:

```text
0
```

The CI system may think:

```text
Deployment successful
```

even though it failed.

Exit codes therefore communicate:

```text
Script
  ↓
CI/CD system
```

whether the operation succeeded.

---

# 35. `$?`

`$?` contains the exit status of the immediately preceding command.

Example:

```bash
mkdir /tmp/test
status=$?

echo "$status"
```

Be careful:

```bash
command
echo "Something"
echo "$?"
```

The final `$?` belongs to:

```bash
echo "Something"
```

not `command`.

Therefore capture immediately:

```bash
command
status=$?
```

---

# 36. `exit`

A script can explicitly terminate:

```bash
exit 0
```

Success:

```bash
exit 0
```

Failure:

```bash
exit 1
```

Example:

```bash
if [[ ! -f "$config" ]]; then
    echo "Configuration missing" >&2
    exit 1
fi
```

---

# 37. `return` vs `exit`

Inside a function:

```bash
return 1
```

returns from the function.

```bash
exit 1
```

terminates the entire script/process.

Therefore:

```text
return → leave function

exit → leave script
```

---

# 38. `set -e`

Now we move into **defensive Bash**.

```bash
set -e
```

is called `errexit`.

The general idea is:

> Exit when an unhandled command fails.

Example:

```bash
set -e

echo "Starting"

cp source.txt destination.txt

echo "Copy complete"
```

If the copy encounters an unhandled failure, the script can stop instead of blindly continuing.

---

# 39. Why `set -e` Is Useful

Without defensive handling:

```text
Command A
   ↓
FAILS
   ↓
Command B
   ↓
Command C
   ↓
"Deployment complete"
```

This can be disastrous.

With `set -e`, the intended behavior becomes closer to:

```text
Command A
   ↓
FAILS
   ↓
STOP
```

This prevents many cascading failures.

---

# 40. Important `set -e` Caveat

`set -e` does **not** literally mean:

> Exit whenever any command returns non-zero.

Bash has contextual rules.

For example:

```bash
if grep -q "ERROR" app.log; then
    echo "Errors found"
fi
```

If `grep` returns:

```text
1
```

that can simply mean:

```text
No match found
```

That is an expected condition.

Similarly:

```bash
command || fallback
```

is explicitly handling failure.

Therefore:

> `set -e` is a safety mechanism, not a replacement for understanding exit statuses.

---

# 41. `set -u`

```bash
set -u
```

is called `nounset`.

It makes references to unset variables an error.

Without it:

```bash
echo "$environment"
```

where `environment` was never defined may silently produce an empty value.

That can be dangerous.

Imagine:

```bash
rm -rf "$TARGET_DIR"
```

If `$TARGET_DIR` unexpectedly becomes empty, you could have a serious problem.

`set -u` helps expose missing variables instead of silently treating them as empty.

---

# 42. Default Values with `set -u`

With strict variable checking, you can intentionally provide defaults.

```bash
environment="${ENVIRONMENT:-staging}"
```

Meaning:

```text
If ENVIRONMENT is set and non-empty:
    use it

Otherwise:
    use staging
```

Another useful form:

```bash
"${VARIABLE:?VARIABLE must be set}"
```

Example:

```bash
environment="${ENVIRONMENT:?ENVIRONMENT must be set}"
```

Now the script explicitly requires the variable.

---

# 43. `set -o pipefail`

Consider:

```bash
command1 | command2 | command3
```

Pipelines connect:

```text
stdout → stdin
```

Example:

```bash
cat app.log | grep ERROR | wc -l
```

Without `pipefail`, the pipeline's status normally follows the final command's status.

That means an earlier failure can be hidden.

---

# 44. The Pipeline Failure Problem

Imagine:

```bash
false | true
```

Results:

```text
false → 1
true  → 0
```

The final command succeeded.

Without `pipefail`, the pipeline can therefore appear successful.

This is dangerous in automation.

---

# 45. `pipefail`

Enable it:

```bash
set -o pipefail
```

Now a pipeline fails when an appropriate command within the pipeline fails.

Conceptually:

```text
false | true

false → FAIL
true  → SUCCESS

pipefail
   ↓
pipeline → FAIL
```

This prevents many hidden failures.

---

# 46. The Famous `set -euo pipefail`

You will see this everywhere:

```bash
set -euo pipefail
```

Breakdown:

```text
-e
│
└── stop on unhandled command failures

-u
│
└── fail on unset variables

-o pipefail
│
└── don't hide failures inside pipelines
```

Think:

```text
          Defensive Bash
                │
       ┌────────┼────────┐
       ▼        ▼        ▼
      -e       -u    pipefail
       │        │        │
    command   missing   pipeline
    failure   variable  failure
```

---

# 47. Why This Is Not "Magic Safe Mode"

This is important.

Even with:

```bash
set -euo pipefail
```

you still need to understand:

* exit codes
* expected failures
* quoting
* globbing
* pipelines
* traps
* permissions
* signals
* race conditions
* idempotency

A bad script with `set -euo pipefail` is still a bad script.

It simply has better failure detection.

---

# 48. Pipelines

A pipeline:

```bash
command1 | command2
```

connects:

```text
command1 stdout
      │
      ▼
command2 stdin
```

Example:

```bash
ps aux | grep nginx
```

Another:

```bash
cat access.log | grep 500 | wc -l
```

---

# 49. Avoid Useless `cat`

This:

```bash
cat file.txt | grep ERROR
```

can usually become:

```bash
grep ERROR file.txt
```

This is called the **Useless Use of Cat** pattern.

It isn't always catastrophic, but direct input is generally clearer.

---

# 50. Standard Streams

Every Unix process has three standard streams:

```text
0 → stdin
1 → stdout
2 → stderr
```

Think:

```text
             PROCESS
           /    |     \
       stdin  stdout  stderr
         0       1       2
```

---

# 51. Redirecting Output

Write stdout to a file:

```bash
command > output.txt
```

Append:

```bash
command >> output.txt
```

Redirect stderr:

```bash
command 2> error.txt
```

Redirect both:

```bash
command > output.txt 2>&1
```

---

# 52. Redirecting stderr to stdout

This:

```bash
command 2>&1
```

means:

```text
stderr
   ↓
stdout
```

So:

```bash
command > output.txt 2>&1
```

sends both stdout and stderr to `output.txt`.

Modern Bash also supports:

```bash
command &> output.txt
```

for Bash-specific scripts.

---

# 53. Send Error Messages to stderr

Good:

```bash
echo "Deployment failed" >&2
```

Why?

Because stdout can be consumed as data.

Example:

```bash
result=$(script.sh)
```

You generally want:

```text
stdout → useful output/data
stderr → diagnostic messages
```

This separation is very useful in automation.

---

# 54. Here-Documents

A here-document allows multiline input.

```bash
cat <<EOF
Hello
This is a multiline block.
EOF
```

Output:

```text
Hello
This is a multiline block.
```

---

# 55. Here-Docs with Variables

```bash
name="Kshitiz"

cat <<EOF
Hello $name
Welcome to DevOps.
EOF
```

Output:

```text
Hello Kshitiz
Welcome to DevOps.
```

Because the delimiter:

```bash
EOF
```

is unquoted.

---

# 56. Literal Here-Docs

Compare:

```bash
cat <<'EOF'
Hello $name
EOF
```

Output:

```text
Hello $name
```

The quoted delimiter:

```bash
'EOF'
```

prevents expansion.

This is useful for generating files containing shell syntax.

For example:

```bash
cat <<'EOF' > script.sh
#!/usr/bin/env bash

echo "$HOME"
EOF
```

The `$HOME` remains literally inside the generated file.

---

# 57. Here-Docs in DevOps

A common pattern:

```bash
cat > /etc/myapp/config.conf <<EOF
environment=$environment
port=$port
region=$region
EOF
```

This allows a script to generate configuration files.

---

# 58. Here-Strings

Another related feature:

```bash
grep "ERROR" <<< "$log"
```

A here-string:

```bash
<<<
```

feeds a string into stdin.

Conceptually:

```text
"$log"
  ↓
stdin
  ↓
grep
```

This is useful when a command expects stdin.

---

# 59. Process Substitution

Process substitution looks like:

```bash
<(command)
```

It allows command output to be presented to another command in a file-like form.

Example:

```bash
diff <(sort file1.txt) <(sort file2.txt)
```

Conceptually:

```text
sort file1
    │
    ▼
 file-like input
    │
    └─────────┐
              ▼
             diff
              ▲
    ┌─────────┘
    │
 file-like input
    ▲
    │
sort file2
```

---

# 60. Why Process Substitution Is Useful

Without it:

```bash
sort file1.txt > /tmp/file1.sorted
sort file2.txt > /tmp/file2.sorted

diff /tmp/file1.sorted /tmp/file2.sorted
```

With process substitution:

```bash
diff <(sort file1.txt) <(sort file2.txt)
```

No explicit temporary files.

---

# 61. Traps

A trap allows you to execute code when a particular event occurs.

Example:

```bash
trap 'echo "Interrupted"' INT
```

Pressing:

```text
Ctrl+C
```

generates `SIGINT`.

The trap can execute your handler.

---

# 62. Important Signals

### SIGINT

Usually generated by:

```text
Ctrl+C
```

### SIGTERM

Requests graceful termination.

This is particularly important for:

```text
Docker
Kubernetes
systemd
```

### SIGHUP

Historically associated with terminal/session hangups.

### EXIT

Bash is exiting.

Example:

```bash
trap cleanup EXIT
```

---

# 63. Cleanup with `trap`

Suppose:

```bash
tmp_file=$(mktemp)
```

We want to delete it when we're finished.

Define:

```bash
cleanup() {
    rm -f "$tmp_file"
}
```

Register:

```bash
trap cleanup EXIT
```

Now:

```text
script starts
    ↓
temporary file created
    ↓
work happens
    ↓
script exits
    ↓
EXIT trap
    ↓
cleanup
```

---

# 64. Why Cleanup Matters

Scripts often create temporary resources:

* temporary files
* directories
* locks
* sockets
* credentials
* generated configuration
* mounted resources

If the script fails halfway through, those resources may remain.

A cleanup trap provides a central place to remove them.

---

# 65. `mktemp`

Use:

```bash
tmp_file=$(mktemp)
```

rather than manually inventing:

```bash
/tmp/myfile
```

Why?

Because manually generated temporary names can create:

* collisions
* race conditions
* security issues

`mktemp` creates a unique temporary file/directory.

---

# 66. Complete Cleanup Pattern

```bash
#!/usr/bin/env bash

set -euo pipefail

tmp_file=$(mktemp)

cleanup() {
    rm -f "$tmp_file"
}

trap cleanup EXIT

echo "Doing work..."

echo "Done"
```

Regardless of normal completion or many forms of failure:

```text
EXIT
 ↓
cleanup
```

will be invoked.

---

# 67. Idempotency

Now we reach one of the most important DevOps concepts.

**Idempotency** means:

> Running an operation multiple times produces the same desired final state as running it once.

Mathematically, conceptually:

```text
f(f(x)) = f(x)
```

In infrastructure:

```text
Apply desired state
Apply desired state again
Apply desired state again
```

should not continuously create additional unintended changes.

---

# 68. Why Idempotency Matters

Imagine a deployment script:

```text
Run once → works
Run twice → breaks
Run three times → makes things worse
```

That is dangerous.

Automation often gets retried because:

* network failed
* CI runner restarted
* deployment timed out
* operator reran the job
* machine rebooted
* previous run's status was unclear

Therefore:

> **Automation must assume it may be run more than once.**

---

# 69. Non-Idempotent Example

Bad:

```bash
mkdir /opt/myapp
```

First run:

```text
directory created
```

Second run:

```text
mkdir: File exists
```

Better:

```bash
mkdir -p /opt/myapp
```

Now:

```text
First run  → create
Second run → already exists, no problem
```

---

# 70. Non-Idempotent File Modification

Bad:

```bash
echo "server=api01" >> /etc/myapp.conf
```

Run once:

```text
server=api01
```

Run again:

```text
server=api01
server=api01
```

Run repeatedly:

```text
server=api01
server=api01
server=api01
...
```

---

# 71. Idempotent Configuration

Instead of blindly adding configuration, determine whether the desired state already exists.

Example:

```bash
if ! grep -q '^server=api01$' /etc/myapp.conf; then
    echo "server=api01" >> /etc/myapp.conf
fi
```

Now:

```text
Run 1
→ add configuration

Run 2
→ configuration already exists

Run 3
→ configuration already exists
```

---

# 72. Desired-State Thinking

This is the most important conceptual shift.

Don't think:

> "What commands do I execute?"

Think:

> **"What should the machine look like after the script finishes?"**

Example:

Desired state:

```text
/opt/myapp exists
nginx installed
config exists
service running
port 8080 listening
```

The script should inspect the current state and move it toward that state.

---

# 73. Idempotency Pattern

Use:

```text
CURRENT STATE
      │
      ▼
Does desired state already exist?
      │
 ┌────┴────┐
 │         │
YES        NO
 │         │
 │         ▼
 │      Make change
 │         │
 └────┬────┘
      ▼
DESIRED STATE
```

---

# 74. Example — User Creation

Non-idempotent:

```bash
useradd appuser
```

If the user already exists, the command fails.

Idempotent approach:

```bash
if ! id appuser >/dev/null 2>&1; then
    useradd appuser
fi
```

Desired state:

```text
appuser exists
```

It doesn't matter whether the script was run:

```text
1 time
10 times
100 times
```

The final state remains:

```text
appuser exists
```

---

# 75. Example — Directory

Desired:

```text
/opt/application
```

Use:

```bash
mkdir -p /opt/application
```

instead of:

```bash
mkdir /opt/application
```

---

# 76. Example — Package

A basic pattern:

```bash
if ! command -v nginx >/dev/null 2>&1; then
    apt-get update
    apt-get install -y nginx
fi
```

Desired state:

```text
nginx command exists
```

Once it exists, repeated execution doesn't reinstall it.

In real production systems, package/configuration management tools such as Ansible are often preferable for larger state-management tasks, but the principle is the same.

---

# 77. Idempotency Is Not the Same as "No Changes"

An idempotent script can absolutely make changes.

First run:

```text
Current state:
nothing

Desired state:
application installed

Action:
install application
```

Second run:

```text
Current state:
application installed

Desired state:
application installed

Action:
nothing
```

The important property is:

```text
Repeated execution
        ↓
same desired final state
```

---

# 78. Idempotency and Kubernetes

This concept will become extremely important when you reach Kubernetes.

A Kubernetes controller effectively thinks:

```text
Observed state
      ↓
Desired state
      ↓
Reconcile
      ↓
Observed state
      ↓
Desired state
```

Your Bash scripts can use the same conceptual approach.

That is one reason learning idempotent scripting is useful before moving deeper into Kubernetes.

---

# 79. Verification

Don't stop after making a change.

Bad:

```bash
systemctl restart nginx
echo "Deployment successful"
```

Better:

```bash
systemctl restart nginx

if systemctl is-active --quiet nginx; then
    echo "Nginx is running"
else
    echo "Nginx failed to start" >&2
    exit 1
fi
```

The sequence becomes:

```text
CHANGE
 ↓
VERIFY
 ↓
SUCCESS / FAILURE
```

---

# 80. Expected Failure vs Unexpected Failure

This distinction is critical with `set -e`.

Some commands return non-zero because something is wrong.

Others return non-zero because they are communicating a perfectly normal condition.

Example:

```bash
grep -q "ERROR" app.log
```

Exit codes:

```text
0 → found
1 → not found
>1 → actual error
```

So:

```bash
if grep -q "ERROR" app.log; then
    echo "Errors found"
fi
```

is appropriate.

You are deliberately handling the result.

---

# 81. Handling Failures Explicitly

Instead of:

```bash
command
```

when failure is expected:

```bash
if ! command; then
    echo "Command failed" >&2
    exit 1
fi
```

This makes the intended control flow obvious.

---

# 82. Logging

Basic logging:

```bash
echo "[INFO] Starting deployment"
echo "[INFO] Deploying application"
echo "[ERROR] Deployment failed" >&2
```

For larger scripts:

```bash
log_info() {
    echo "[INFO] $*"
}

log_error() {
    echo "[ERROR] $*" >&2
}
```

Then:

```bash
log_info "Starting deployment"
log_error "Deployment failed"
```

---

# 83. A Production-Style Bash Structure

A useful structure:

```bash
#!/usr/bin/env bash

set -euo pipefail

cleanup() {
    ...
}

validate_inputs() {
    ...
}

check_prerequisites() {
    ...
}

prepare() {
    ...
}

deploy() {
    ...
}

verify() {
    ...
}

main() {
    validate_inputs
    check_prerequisites
    prepare
    deploy
    verify
}

trap cleanup EXIT

main "$@"
```

This isn't mandatory, but it gives you a strong structure for larger automation.

---

# 84. Complete Example

Here is a small but realistic example.

```bash
#!/usr/bin/env bash

set -euo pipefail

APP_DIR="/opt/myapp"
CONFIG_FILE="$APP_DIR/config.conf"

cleanup() {
    echo "[INFO] Cleanup complete"
}

log_info() {
    echo "[INFO] $*"
}

log_error() {
    echo "[ERROR] $*" >&2
}

validate_inputs() {
    if [[ $# -lt 1 ]]; then
        log_error "Usage: $0 <environment>"
        exit 1
    fi
}

prepare() {
    log_info "Ensuring application directory exists"

    mkdir -p "$APP_DIR"
}

configure() {
    if [[ ! -f "$CONFIG_FILE" ]]; then
        log_info "Creating configuration"

        cat > "$CONFIG_FILE" <<EOF
environment=$1
EOF
    else
        log_info "Configuration already exists"
    fi
}

verify() {
    if [[ -f "$CONFIG_FILE" ]]; then
        log_info "Configuration verified"
    else
        log_error "Configuration missing"
        return 1
    fi
}

main() {
    validate_inputs "$@"

    local environment="$1"

    log_info "Starting deployment"
    log_info "Environment: $environment"

    prepare
    configure "$environment"
    verify

    log_info "Deployment complete"
}

trap cleanup EXIT

main "$@"
```

Run:

```bash
./deploy.sh production
```

The script demonstrates:

```text
shebang
strict mode
functions
arguments
local variables
quoting
stderr
here-doc
idempotent directory creation
idempotent configuration creation
verification
cleanup
trap
exit codes
```

---

# 85. What Happens When This Script Runs?

```text
./deploy.sh production
        │
        ▼
main "$@"
        │
        ▼
validate_inputs
        │
        ▼
environment="production"
        │
        ▼
prepare
        │
        ▼
mkdir -p /opt/myapp
        │
        ▼
configure
        │
        ├── config exists?
        │       │
        │       ├── YES → leave it
        │       │
        │       └── NO → create it
        │
        ▼
verify
        │
        ▼
success
        │
        ▼
EXIT trap
        │
        ▼
cleanup
```

---

# 86. What If Something Fails?

Suppose:

```bash
mkdir -p "$APP_DIR"
```

fails.

Because:

```bash
set -e
```

is enabled, the script doesn't blindly continue.

Then Bash exits and:

```bash
trap cleanup EXIT
```

runs the cleanup handler.

Conceptually:

```text
Command fails
     │
     ▼
set -e
     │
     ▼
script exits
     │
     ▼
EXIT trap
     │
     ▼
cleanup
```

This is the pattern you want to understand.

---

# 87. Bash and Containers

Everything you've learned here becomes especially important in containers.

A Dockerfile may contain:

```dockerfile
ENTRYPOINT ["/app/entrypoint.sh"]
```

and:

```bash
entrypoint.sh
```

might:

```text
read environment variables
        ↓
validate configuration
        ↓
generate config
        ↓
start application
```

If the script is poorly written:

```text
container starts
     ↓
configuration error
     ↓
script ignores failure
     ↓
application starts incorrectly
```

Or worse:

```text
container appears healthy
but application is broken
```

---

# 88. Bash and CI/CD

A pipeline might execute:

```bash
./build.sh
./test.sh
./deploy.sh
```

The CI system relies heavily on exit codes:

```text
build.sh → 0
     ↓
test.sh → 0
     ↓
deploy.sh → 0
     ↓
pipeline SUCCESS
```

If:

```text
test.sh → 1
```

the pipeline should fail.

This is why understanding:

```text
exit codes
set -e
pipefail
```

is critical.

---

# 89. Bash and Kubernetes

Eventually you'll encounter:

```text
Pod
 ↓
Container
 ↓
ENTRYPOINT
 ↓
Bash script
 ↓
Application
```

Signals become important too.

For example:

```text
Kubernetes
    │
    │ SIGTERM
    ▼
Container process
    │
    ▼
Graceful shutdown
```

This connects directly to the Linux process/signal concepts you studied earlier.

---

# 90. Common Bash Mistakes to Avoid

## Mistake 1 — Unquoted variables

Bad:

```bash
rm $file
```

Prefer:

```bash
rm "$file"
```

---

## Mistake 2 — Unsafe array expansion

Bad:

```bash
for item in ${arr[@]}; do
```

Prefer:

```bash
for item in "${arr[@]}"; do
```

---

## Mistake 3 — Ignoring exit codes

Bad:

```bash
deploy
echo "Deployment complete"
```

when `deploy` can fail.

Use appropriate error handling.

---

## Mistake 4 — Assuming `set -e` solves everything

It doesn't.

Understand expected failures and Bash's `errexit` rules.

---

## Mistake 5 — Ignoring pipeline failures

Bad:

```bash
set -e
command1 | command2
```

Better defensive baseline:

```bash
set -euo pipefail
```

---

## Mistake 6 — Hard-coded temporary filenames

Bad:

```bash
/tmp/myfile
```

Prefer:

```bash
mktemp
```

---

## Mistake 7 — Scripts that cannot be rerun

Bad:

```bash
mkdir /opt/app
echo "config" >> /opt/app/config
useradd appuser
```

Better:

```text
Check desired state
       ↓
Change only when necessary
```

---

## Mistake 8 — No verification

Don't assume:

```bash
command succeeded
```

means:

```text
system is now correct
```

Verify the desired state.

---

# 91. The Bash Rules Worth Memorizing

### Rule 1

Quote variable expansions:

```bash
"$var"
```

### Rule 2

Use:

```bash
"${arr[@]}"
```

for arrays.

### Rule 3

Use:

```bash
"$@"
```

when forwarding script arguments.

### Rule 4

Understand exit codes:

```text
0 = success
non-zero = failure
```

### Rule 5

Capture `$?` immediately if needed.

### Rule 6

Use:

```bash
set -euo pipefail
```

as a strong defensive baseline.

### Rule 7

Don't treat `set -e` as magic.

### Rule 8

Use traps for cleanup.

```bash
trap cleanup EXIT
```

### Rule 9

Use `mktemp` for temporary resources.

### Rule 10

Think in desired state.

### Rule 11

Make scripts safe to rerun.

### Rule 12

Verify changes.

---

# 92. The Bigger Picture

The progression you're learning is intentional:

```text
Bash syntax
     ↓
Variables
     ↓
Quoting
     ↓
Arguments
     ↓
Arrays
     ↓
Conditions
     ↓
Loops
     ↓
Functions
     ↓
Exit codes
     ↓
Pipelines
     ↓
set -euo pipefail
     ↓
Traps
     ↓
Cleanup
     ↓
Idempotency
     ↓
Desired-state thinking
```

This takes you from:

> "I know some Linux commands."

to:

> **"I can build reliable Linux automation."**

That distinction matters considerably in DevOps.

---

# 93. Final Mental Model

When writing a Bash automation script, ask:

```text
┌────────────────────────────────────────────┐
│           BEFORE WRITING CODE              │
├────────────────────────────────────────────┤
│                                            │
│ What inputs do I have?                     │
│                                            │
│ What assumptions am I making?              │
│                                            │
│ What is the desired final state?           │
│                                            │
│ What happens if a command fails?            │
│                                            │
│ Which failures are expected?               │
│                                            │
│ Am I quoting variables?                    │
│                                            │
│ Can the script be run twice safely?        │
│                                            │
│ What resources need cleanup?               │
│                                            │
│ How will I verify success?                 │
│                                            │
│ What exit code should I return?             │
│                                            │
└────────────────────────────────────────────┘
```

That checklist is more valuable than memorizing hundreds of Bash commands.

---

# Revision Cheat Sheet

```text
============================================================
                 BASH DEVOPS CHEAT SHEET
============================================================

SCRIPT
------
#!/usr/bin/env bash


VARIABLE
--------
name="Kshitiz"

Use:
"$name"


COMMAND SUBSTITUTION
--------------------
result="$(command)"


QUOTING
-------
"$var"
→ preserve variable as one word

"${arr[@]}"
→ preserve array elements separately

"$@"
→ preserve script arguments separately

"${var}_suffix"
→ safely concatenate variable + text


SINGLE VS DOUBLE QUOTES
-----------------------
"$var"
→ variable expands

'$var'
→ literal "$var"


ARGUMENTS
---------
$0 → script
$1 → first argument
$2 → second argument
$# → argument count
"$@" → all arguments


ARRAY
-----
arr=("one" "two words" "three")

"${arr[0]}"

for item in "${arr[@]}"; do
    echo "$item"
done


CONDITIONS
----------
if [[ "$x" == "value" ]]; then
    ...
fi

-f → file
-d → directory
-x → executable
-z → empty
-n → non-empty


FUNCTION
--------
my_function() {
    local value="$1"
    ...
}


EXIT CODES
----------
0      → success
nonzero → failure

status=$?

exit 0
exit 1

return 1
→ return from function

exit 1
→ terminate script


STRICT MODE
-----------
set -euo pipefail

-e
→ fail on unhandled command failures

-u
→ fail on unset variables

-o pipefail
→ pipeline failures propagate


PIPELINE
--------
command1 | command2 | command3

pipefail prevents earlier failures
from being hidden by a successful
last command.


STD STREAMS
-----------
0 → stdin
1 → stdout
2 → stderr

command > file
command >> file
command 2> error
command > file 2>&1


ERROR OUTPUT
------------
echo "error" >&2


HERE-DOC
--------
cat <<EOF
hello $name
EOF

→ variables expand


LITERAL HERE-DOC
----------------
cat <<'EOF'
hello $name
EOF

→ variables do not expand


HERE-STRING
-----------
grep "ERROR" <<< "$log"


PROCESS SUBSTITUTION
--------------------
diff <(command1) <(command2)

Command output behaves like file input.


TRAPS
-----
trap cleanup EXIT

trap handler INT
trap handler TERM


TEMP FILES
----------
tmp_file="$(mktemp)"

cleanup() {
    rm -f "$tmp_file"
}

trap cleanup EXIT


IDEMPOTENCY
-----------
Think:

CURRENT STATE
      ↓
DESIRED STATE
      ↓
MAKE NECESSARY CHANGE
      ↓
VERIFY


GOOD:
mkdir -p /opt/app

BAD:
mkdir /opt/app


GOOD:
check whether configuration exists
before adding it

BAD:
blindly append every execution


EXPECTED FAILURE
----------------
if grep -q "ERROR" file; then
    ...
fi

Do not assume every non-zero
exit code means catastrophic failure.


DEVOPS SCRIPT FLOW
------------------
INPUT
  ↓
VALIDATE
  ↓
INSPECT
  ↓
DESIRED STATE
  ↓
CHANGE
  ↓
VERIFY
  ↓
CLEANUP
  ↓
EXIT


GOLDEN RULE
-----------
A good Bash script should not merely
"work once."

It should:

• fail predictably
• handle expected failures
• preserve arguments correctly
• clean up resources
• return meaningful exit codes
• verify its work
• be safe to rerun
• converge toward the desired state
```

---

# What You Should Practice

Before moving to the next topic, build **one real script** that combines everything:

```text
deploy.sh

Input:
    environment

Requirements:
    • set -euo pipefail
    • validate environment
    • use functions
    • use an array
    • create a temporary file
    • use a trap for cleanup
    • generate a configuration using a here-doc
    • use a pipeline
    • verify the generated configuration
    • make directory creation idempotent
    • make configuration creation idempotent
    • return a meaningful exit code
```

Once you can write that without copying a template, you've moved beyond **Bash syntax** into the kind of shell scripting that actually becomes useful in DevOps.

## Suggested file name

```text
ops_arsenal/
└── notes/
    └── bash-scripting-foundations.md
```

And I'd keep the **cheat sheet at the bottom of the same file**, rather than creating a separate file yet. That gives you one authoritative reference to maintain as we continue building your DevOps knowledge.
