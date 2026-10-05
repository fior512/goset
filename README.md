[![License: Apache 2.0](https://img.shields.io/badge/license-Apache%202.0-blue.svg)](LICENSE)


GOSET : Noise-aware CPU pinning
============================================

<img src="images/logo/goset.png" width="40%" style="float: right">

Goset is a systematic CPU-pinning CLI tool.
It takes a task, single|multi threaded, ranks cores by interrupt rate and scheduler noise, isolates the task, steers future interrupts and reports kernel and hardware counters for the run.

* **Noise-aware:** Ranks CPUs by IRQ rate and scheduler load before pinning.
* **Isolated:** cgroup v2 + IRQ steering, for exclusive CPU access.
* **Telemetry built in:** IRQ, throttle, ctxsw counts, ...; reported with the run.
* **No IPI:** Housekeeper thread reads every CPU's counters remotely, from kernel-maintained state.
* **Diagnostics:** Reports topology, environment, existing cgroups.

Table of Contents
-----------------

* [Quick Start](#quick-start)
* [Technical insights](#documentation)
* [Performance results](#performance-results)
* [Contributing](CONTRIBUTING.md)
* [AI Policy](AGENTS.md)
* [License](LICENSE)


Quick Start
-----------

Goset needs sudo only for `-cgroup` and `-steer`.

| flag | type | default | description | rule |
|---|---|---|---|---|
| ` -- ` |string| | Separating Goset flags and task | |
| `-n` |int| 1 | How many threads to book | n>=1 |
| `-cgroup` |bool| false | Containerize task inside Cgroupv2, holds tasks that reset their own affinity | **sudo** |
| `-fence` |bool| false | Books the SMT siblings of the selected CPUs in the cgroup, idle. | **sudo**, `-cgroup` |
| `-steer` |bool| false | Push away steerable IRQs, automatically handle IRQBalance | **sudo** |
| `-interval` |int| 100 | Poll interval in milliseconds for telemetry collection | |
| `-include` |string| | List of threads to select first, handles ranges (e.g.: `1,3-5` -> 1,3,4,5)| |
| `-exclude` |string| | List of threads to avoid, handles ranges (e.g.: `1,3-5` -> 1,3,4,5) | |
| `-numa` |int| -2 | Constrain the task cpus and memory to one node: 0..N that node only, -1:Auto (widest node), -2:Off (multi node). A node that cannot fill `-n` is an error. | |

> Multi-thread tasks: `-n >1` works without `-cgroup`, the mask is inherited by every thread. `-cgroup` adds exclusivity and holds tasks that reset their own affinity.


Goset has a second form called `Diagnostic`, callable with bare `goset`.

| flag | description | rule |
|---|---|---|
| *bare* | Report Topology, CPU state, cgroups.. | | 
| `-rm-cgroup` | Let you **brut-force delete** an existing group. In case of non-identified goset bug | **sudo** |

> Identifying Goset's cgroup: name is "goset-" + the task binary's basename (e.g. task: `./mybench arg1, arg2` -> cgroup name: `goset-mybench`).

> Concurrent runs: each cgroup name and IRQ steering are guarded by a lock under `/run/lock`. A second run that would reuse the same cgroup or steer at the same time fails immediately and reports the holder pid.

**Real usage**:
```bash
# Pin to one quiet CPU
goset -n 1 -- ./task

# Request 4 threads with full isolation
sudo goset -n 4 -cgroup -steer -fence -- ./task

# Steer selection toward 2,4,5,6 and avoid 0 and 1 for remaining N
goset -n 5 -numa 0 -include 2,4-6 -exclude 0,1 -- ./task

# Change the telemetry sampling window
goset -interval 500 -- ./task


# Remove a cgroup goset left behind (in case of bug)
sudo goset -rm-cgroup goset-mybench
```

**Output**:

Goset output format (with `-steer` and `-cgroup`):
```
----------------- GOSET -----------------
Selection
  cpu  sel  steer  non-steer   core  sibl  isol  numa  nohz  rcu
    7   *       2        306    673     1           0
    1   *     239        367    673     7           0
    8   &       4        297    685     2           0
    2           0        388    685     8           0
    3           0        373    842     9           0
    9           0        469    842     3           0
    6           0        408    909     0           0
    0           0        501    909     6           0
    4           0        456    964    10           0
   10           0        508    964     4           0
   11          94        522  1.06k     5           0
    5           0        535  1.06k    11           0

Telemetry
  cpu  freq min  freq avg  freq max  throttle count  irq steerable  irq non-steerable
  1     2.99GHz   2.99GHz   2.99GHz               0              0                147
  7     2.99GHz   2.99GHz   2.99GHz               0              0                 13
  all   2.99GHz   2.99GHz   2.99GHz               0              0                160

Run
  task             sched             steer
  poll  10@100ms  ctxsw vol      2  applied         40
  wall     1.00s  ctxsw invol    0  rejected        26
  exit          0  migrations     0  remaining        0
                   run_delay    0ns  drift            0
                                      irqbalance  absent

```



Selection columns:
* `sel`: `*` task cpu, `&` housekeeper, `!` fenced sibling (`-fence`).
* `steer`, `non-steer`: interrupts counted on the cpu during the 500ms ranking sample. Steerable ones can be moved away, non-steerable (NMI, LOC, RES) cannot.
* `core`: rank cost, this cpu's noise plus its sibling's. Steerable interrupts count only without `-steer`.
* `sibl`: SMT sibling cpu, `-` when alone.

The other tables: `Telemetry` holds per-cpu counters over the run, `Run` holds wall time, exit code, scheduler, steer and drift counters.

Technical insights
-------------

**Selection** (`internal/cpu/bench.go`)
- Cores rank by the noise of both threads, quietest first, a core's threads together.
- The task takes the top cpus, the housekeeper the next one outside the task's core.
- With `-fence`, the siblings of the task cpus are booked idle while a cpu stays free for the housekeeper and, with `-steer`, for the IRQs. Siblings left open are reported.

**Isolation**
| mode | mechanism | scope |
|---|---|---|
| default | `sched_setaffinity` | N threads, shared mask |
| `-cgroup` | cgroupv2 cpuset, placed at clone (`CLONE_INTO_CGROUP`) | N threads |

Without `-cgroup`, the CPU mask is inherited by every thread the task starts. A task that resets its own affinity (OpenMP, MPI) escapes it. Use `-cgroup` to hold it.

With `-cgroup`, the cpuset holds the task CPUs plus their SMT siblings (`-fence`), and the task is pinned to the task CPUs by `sched_setaffinity`. A busy sibling halves the throughput of an FP-bound task.

**IRQ steering** (`-steer`)
- Holds `irqbalance` for the run: stops the systemd unit, restarts it on exit.
- Unmanaged daemon (no systemd): warns and steers anyway.
- Writes `/proc/irq/*/smp_affinity_list` for IRQs on selected CPUs.
- Restores prior affinity on exit.

**Telemetry**
- Reader: Only the housekeeper reads.
- No IPI: every counter comes from procfs/sysfs. The pinned housekeeper thread does the mid-run poll; the before/after baseline runs on the calling thread.
- Source: kernel software counters only: `/proc/interrupts`, sysfs throttle, `wait4()` rusage...
- Every counter is reported, so `0` is ambiguous: no activity, no kernel support (a driver or `CONFIG_` option is missing), or nothing was sampled. `-interval 0` samples nothing, so the `freq` columns read 0. A read that failed ends the report in an `Errors` table, one line per failure.
- `-interval`: mid-run poll tick, ms. Not used by `SelectCPUs()`.


Performance results
--------------------

Setup: Ryzen 5 7600X (6 cores, SMT on), 10 interleaved runs per mode, 5s idle before each run.

- baseline: bare binary, scheduler places it.
- taskset: `taskset -c` on one CPU, rotating over CPUs.
- goset: `-n 1 -cgroup -steer -fence`.

Benchmarks:
- jitter: one dependent xorshift-multiply chain. Exposes CPU time lost to interrupts and preemption.
- chase: dependent loads over a 192 KiB ring that fits L2. Exposes core migration and cache pollution.
- matmul: 32x32 double matrix multiply, L1-resident. Exposes CPU frequency and sibling-thread interference.
- stream: STREAM Triad over 20M-element arrays. Exposes memory-bandwidth variation.

**jitter** (ms per iteration, lower is better)

| mode | median | sd | min | max | spread |
|---|---|---|---|---|---|
| baseline | 5.05 | 0.059 | 4.98 | 5.14 | 3.2% |
| taskset | 5.14 | 0.059 | 5.00 | 5.17 | 3.3% |
| goset | 5.00 | 0.042 | 4.96 | 5.10 | 2.8% |

**chase** (ns per load, lower is better)

| mode | median | sd | min | max | spread |
|---|---|---|---|---|---|
| baseline | 2.61 | 0.05 | 2.52 | 2.70 | 6.9% |
| taskset | 2.63 | 0.04 | 2.59 | 2.70 | 4.2% |
| goset | 2.54 | 0.02 | 2.51 | 2.56 | 2.0% |

**matmul** (MFLOP/s, higher is better)

| mode | median | sd | min | max | spread |
|---|---|---|---|---|---|
| baseline | 17241 | 597 | 15803 | 17689 | 10.9% |
| taskset | 16556 | 736 | 14856 | 17434 | 15.6% |
| goset | 18191 | 336 | 17134 | 18352 | 6.7% |

**stream Triad** (MB/s, higher is better)

| mode | median | sd | min | max | spread |
|---|---|---|---|---|---|
| baseline | 40826 | 363 | 40182 | 41367 | 2.9% |
| taskset | 40645 | 395 | 40136 | 41415 | 3.1% |
| goset | 41156 | 233 | 40896 | 41799 | 2.2% |

> `spread` is (max - min) / median

Limits:
- taskset changes core every run, so its spread includes core-to-core differences.
- jitter, chase and stream medians differ by under 4% between modes.

Reproduce: `sudo goset-bench -bench jitter -runs 10 -n 1 -settle 5s -fence=true -taskset=true`.

### Your own task

`goset-bench` runs any command after `--` in each mode, 10 interleaved runs by default, and compares the numbers the task prints.

```
sudo goset-bench -runs 10 -n 1 -settle 5s -taskset=true -- ./mytask arg1 arg2
```

- Every number in the task's output becomes a metric, keyed by its line label and column.
- `-dump` prints the extracted keys. `-match REGEX` selects keys. `-topk N` caps the keys shown (default 8).
- A task that exits non-zero has its run discarded.
- The task must print the same line structure every run. A changed structure raises an `output drift` warning.

> goset-bench extracts metrics from any output format, so unusual structures can yield wrong metrics. Feel free to open an issue with the task's output when that happens!!


License
-------

Apache-2.0. Copyright 2026. See [LICENSE](LICENSE).
