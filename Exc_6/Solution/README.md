# Exercise 6 — *Card Sentinel*: Solution

Erwin Omar Saavedra
**GitHub:** Erwinomar

## 1. Decision: how do the payments reach the models?

I choose log-based CDC with Debezium, because it is the only option that meets R1 and R2 at the same time: it reads PostgreSQL's write-ahead log, so the certified payment path is not changed and the table gets no extra queries, and it delivers every committed payment in commit order. Polling would miss payments that commit out of id order and add load for every agent, dual write changes the payment path and can lose or invent payments if it crashes between the two writes, and the outbox is correct but also changes the payment path. The cost of CDC is the replication slot: PostgreSQL keeps all WAL that Debezium has not read yet, so if the connector is down for a long time the WAL fills the disk, and PostgreSQL, and with it the payment path, stops. In production I would monitor the WAL retained by the slot (pg_replication_slots) and the free disk space, and alert as soon as the connector is not RUNNING. If our team owned the payment path, I would choose the outbox pattern: it is just as reliable, because the payment and its event are written in one transaction, but we design the event and its key ourselves instead of making the table structure the interface for every consumer.

## 2. The three runs

### Run 1: Debezium's defaults



| |                                                                                                                                                                                                                                                                                                                                                                               |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Prediction** | *Debezium is not told how many partitions to create, so I expect only one partition. Then only one of the three velocity instances can read it, the other two stay idle, and one instance cannot handle 1,000 payments/s, so the lag grows and R4 fails.* |
| **Partitions / key (Kafka UI)** | *1 partition; key = {"id": 1328956}, the payment’s primary key (Debezium’s default)*                                                                                                                                                                                                                                                                                          |
| **Agents output** | *vel-1: assigned partitions [0]; vel-2 and vel-3: assigned partitions [] (they never print, because they get no payments). vel-1 processed ~430 payments/s (39.5k payments while ano-1 processed 105.6k), and its lag grew steadily from 2.2 s to 53.9 s (≈ +0.6 s every second). ano-1 kept up with a stable lag of ~0.5 s.*                                                 |

**Scoreboard, velocity lines:**

```text
Run 1794919637: started 13:47:17, 60,123 transactions in 60 s (1,002/s), 4 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   1        1     3,490
  velocity-agent           fraud.public.transactions                   1        3    94,127

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1          28292 anomaly          59,393       44,754             0      1.0       2s ago
  vel-1          19056 velocity         23,324       20,787             0    799.9       3s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          0/2  (0%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  no alerts; worst lag of the velocity agents 799.9 s   FAIL  (<= 5 s)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          2/2  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  1.0 s / 1.1 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

*he topic has only 1 partition, and within a consumer group one partition is read by exactly one instance, so vel-1 did all the work and vel-2 and vel-3 got `assigned partitions []` (scoreboard: 3 members, 1 partition). vel-1 manages only ~430 of 1,000 payments/s, so its lag grew every second and R4 fails; the frauds were still waiting in the backlog, so none were caught. The anomaly model passes because a single instance at 0.3 ms per payment can handle ~2,500 payments/s.*

### Run 2: 6 partitions

**Change:** Added `topic.creation.default.partitions = 6` and `topic.creation.default.replication.factor = 1`.

| |                                                                                                                                                                                                                                                                         |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Prediction** | *we increased the number of partitions to 6 therefore we have enought partitions to assign for every velocity agent so the lag should greatly decrease and therefore the performance should increase a lot too*                                                         |
| **Partitions / key (Kafka UI)** | *6 partitions; key = {"id": 1023757}, the payment’s primary key (Debezium’s default)*                                                                                                                                                                                   |
| **Agents output** | *vel-1 [0, 1], vel-2 [2, 3], vel-3 [4, 5]; lag stable at 0.5–1.1 s. After ~68 s: 0 velocity alerts (ano-1: 2). The three velocity instances saw 58.4k cardholders together but only 49.3k exist so far, so the same cardholders’ payments were split across instances.* |

**Scoreboard, velocity lines:**

```text
Run 1794920337: started 13:58:57, 120,172 transactions in 120 s (1,001/s), 12 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1       600
  velocity-agent           fraud.public.transactions                   6        3       600

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1           5032 anomaly         119,173       69,715             0      0.7       2s ago
  vel-1           9984 velocity         39,887       32,913             2      0.7       5s ago
  vel-2          29152 velocity         39,913       32,893             0      0.6       0s ago
  vel-3          33492 velocity         40,019       33,119             2      0.7       0s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          0/6  (0%)   FAIL  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  no alerts; worst lag of the velocity agents 2.6 s   PASS  (<= 5 s)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          6/6  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.6 s / 0.9 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

*the key is the id of the transaction because of this every transaction is being deliver in basically random partitions that are being sent to the models indepently of each other , because of this the readings are spread among many different partitions and therefore fail to detect the fraud in the velocity model,ano-1 still passes because it is a single instance reading all six partitions, so it sees every payment of every cardholder. With two anomaly instances, it would fail the same way.*

### Run 3: the cardholder as the key

**Change:** Added `"message.key.columns": "public.transactions:user_id"`

| |                                                                                                                                                                                                                                                                                     |
|---|-------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------|
| **Prediction** | *by changing the key to be user_id kafka is going to assign every user order to the same partition that's going to be read at the time they were comited , therefore the detector is going to read succesfully and detect if there is a faulty by solving the previous run problem* |
| **Partitions / key (Kafka UI)** | *`6 partitions; key = {"user_id":166740}` the card holder*                                                                                                                                                                                                                          |
| **Agents output** | *vel-1 [0, 1], vel-2 [2, 3], vel-3 [4, 5]; each processed ~45k payments; lag stable at ~0.7 s. The three velocity instances saw ~25k cardholders each, together ~75.8k ≈ the 75.4k ano-1 saw → each cardholder goes to exactly one instance.*                                       |

**Scoreboard, velocity lines:**

```text
Run 1794920589: started 14:03:09, 60,123 transactions in 60 s (1,002/s), 4 frauds so far (still running)

Consumer groups on the CDC topic(s)   [R2]
  group                    topic                              partitions  members       lag
  anomaly-agent            fraud.public.transactions                   6        1     4,603
  velocity-agent           fraud.public.transactions                   6        3     4,707

Agent instances (telemetry of this run)   [R3, R4]
  instance         pid model      transactions  cardholders  out-of-order  lag (s)      seen
  ano-1           5308 anomaly          56,622       43,204             0      0.8       4s ago
  vel-1          19844 velocity         19,106       14,942             0      0.8       5s ago
  vel-2           4716 velocity         17,768       13,743             0      0.8       5s ago
  vel-3          26048 velocity         18,053       13,884             0      0.8       4s ago

Alerts on 'fraud-alerts' (committed, this run)

  velocity model (card testing)
    R3  frauds caught          2/2  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.8 s / 1.0 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)

  anomaly model (account takeover)
    R3  frauds caught          2/2  (100%)   PASS  (>= 90 %)
    R3  false alerts           0   PASS  (at most 10 % of the flagged transactions)
    R4  alert latency p50/p95  0.7 s / 0.9 s   PASS  (p95 <= 5 s after the commit)
        duplicate alerts       0   (info: alerts case management got twice)
```

*With `user_id` as the key, all payments of one cardholder land in the same partition and are read by the same instance in commit order, so that instance sees the whole attack and alerts at the 8th payment (R3 passes). The 100,000 cardholders are still spread evenly over the 6 partitions, so all instances keep up (R4 passes). There were no false alerts in this run, but an honest cardholder who pays 8 times within a minute would be flagged; R3 allows a few of these, because a threshold rule cannot tell them apart from a thief.*

## 3. Closing: defaults are decisions

*Debezium chose two defaults: 1 partition, and the primary key (`id`) as the key.*
- *What would have happened in production if nobody had checked them? There would have been a bottleneck in the velocity model, leading to high lag and failure to detect frauds, therefore the frauds wouldn't be detected on time.*

- *How could the team have noticed? for example using the lag to detect the bottle neck and the detection rate for the frauds or the detection delay could let us know there is a problem*


If nobody had checked the defaults, the pipeline would have looked fine but silently failed: with 1 partition the extra velocity instances would sit idle and the alerts would come minutes late, and even after adding partitions, the primary key as the key would split every cardholder over several instances, so card-testing frauds would simply not be caught. Nothing crashes and no error appears, so the only sign is fewer caught frauds. The team could have noticed through growing consumer lag, idle consumers (`assigned partitions []`, more members than partitions), alert latency, and by comparing the number of detected frauds with the frauds later confirmed by chargebacks.
