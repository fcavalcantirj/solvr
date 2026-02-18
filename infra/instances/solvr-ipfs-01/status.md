# solvr-ipfs-01 — IPFS Node Status

**Provisioned:** 2026-02-17
**Location:** hel1 (Helsinki)
**Type:** cx33 (4 vCPU, 8GB RAM, 80GB SSD)
**Cost:** ~€5.49/mo

## Connection

- **IP:** 65.109.134.87
- **SSH:** `ssh -i ~/.ssh/solvr_solvr-ipfs-01 root@65.109.134.87`
- **IPFS API:** `http://127.0.0.1:5001` (via SSH tunnel)
- **IPFS Gateway:** `http://127.0.0.1:8080` (via SSH tunnel)
- **P2P Port:** 4001 (public)

## IPFS Details

- **Peer ID:** `12D3KooWJG6rZ1KWTQy1fPeaZuxhfukik3RmYTjyf76Yn6CwUP3A`
- **Version:** Kubo 0.39.0
- **Storage Max:** 10GB (configurable)
- **Swarm Peers:** 312+ (healthy)

## SSH Tunnel (for API access)

```bash
# Open tunnel to access IPFS API locally
ssh -i ~/.ssh/solvr_solvr-ipfs-01 -L 5001:127.0.0.1:5001 -L 8080:127.0.0.1:8080 -N root@65.109.134.87

# Then in another terminal:
curl http://localhost:5001/api/v0/id
```

## Quick Commands

```bash
# Check status
cd ~/development/solvr && ./scripts/infra.sh ipfs-status solvr-ipfs-01

# SSH into server
cd ~/development/solvr && ./scripts/infra.sh ssh solvr-ipfs-01

# Open tunnel
cd ~/development/solvr && ./scripts/infra.sh ipfs-tunnel solvr-ipfs-01
```

## Next Steps

1. Configure Solvr backend to connect (via internal network or SSH tunnel)
2. Set up storage quotas
3. Add monitoring/alerting
