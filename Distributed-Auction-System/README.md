# Ricart-Agrawala Mutual Exclusion Algorithm

Distributed mutual exclusion using Lamport timestamps and gRPC.

## Running the Program

**Important: You MUST run 3 instances in 3 separate terminals.**

1. **Clear PeerList.txt:**
   ```bash
   truncate -s 0 PeerList.txt
   ```

2. **Terminal 1:**
   ```bash
   go run main.go
   ```

3. **Terminal 2:**
   ```bash
   go run main.go
   ```

4. **Terminal 3:**
   ```bash
   go run main.go
   ```

The processes will automatically discover each other and start requesting the critical section. Only one process will be in the critical section at a time, ordered by Lamport timestamps.

## Stopping

Press `Ctrl+C` in each terminal to stop.

