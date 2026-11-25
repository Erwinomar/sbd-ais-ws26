# Exercise 7: From the log to the lake — Solution

## 1. Decision: which architecture and why

**Chosen architecture:** B — The lake is the truth


- from all the architectures I chose to use B because for me it was the clearest and most logical to implement , we make a temporal folder called `STAGING` where we write the new records and then we move them to the `LAKE` folder, this way we avoid duplicates and missing records ,also we read from the `LAKE` folder to know where we left off, this way we avoid having to save a state file or having to deduplicate the records when reading.


- Each run is written to `STAGING` first (nobody reads it), then moved into `LAKE` with
  `publish(run)` — **a single step**. Readers see either the whole or nothing.

- Readers don't need a special deduplication view (unlike C), the lake is always clean.


- Why not A: if the job stops after writing but before save_state, the next run reads the same records again duplicates so possible to end up with duplicates in the lake, also if the state file is lost or corrupted, the job will duplicate records.
- Why not C: C allows duplicates in the lake , it's solution is to just filter them with a view at the readers side.Every query pays the cost of removing them, and the lake keeps growing with duplicate data.

### Implementation

```python
def run_once():
    start = lake_position()           # 1. where did the last run stop?
    new = read_new(start)             # 2. read what is new
    run = write_files(new, STAGING)   # 3. write it where nobody reads
    publish(run)                      # 4. one rename: the run becomes visible
```

## 2. Explain the result

**Why the example failed:**
It reads from the beginning (None) every time and writes straight into the lake,
     so the second run copies all records again so every record twice. 

**Why my solution passes each situation:**

| Situation | What happens                                                                                                                               |
|---|--------------------------------------------------------------------------------------------------------------------------------------------|
| First run | Lake is empty then lake postion = NONE so it just reads all the next records                                                               |
| New records | Starts at the end of the last recoreded record and just continues recording                                                                |
| Nothing new | Start end is empty so nothing happens                                                                                                      |
| Stops **while** writing | Partial files only in staging so lake unchanged, next run starts from the same position and writes a new run folder and publishes that one |
| Stops **after** writing | Files in STAGING but not published so not visible therefore next run writes it again from the same position                                |

Leftover folders in STAGING after a crash are harmless (nobody reads them) and could be cleaned up.

## 3. Output of `check_all`

```
Situation                                 in lake  duplicates  missing  not loaded    run  check
-----------------------------------------------------------------------------------------------
1 first run                             6,564,000           0        0           0  97.4s  PASS
2 new records arrive, run again         6,664,000           0        0           0   6.8s  PASS
3 nothing new, run again                6,664,000           0        0           0   0.5s  PASS
4 run stops WHILE writing: right after  6,664,000           0        0     100,000         PASS
  ... then the next run                 6,764,000           0        0           0   6.7s  PASS
5 run stops AFTER writing: right after  6,764,000           0        0     100,000         PASS
  ... then the next run                 6,864,000           0        0           0   6.7s  PASS
-----------------------------------------------------------------------------------------------
Exactly once after every run: YES
```
