# solvr-ipfs-01 — Hardening Report

**Date:** 2026-02-17
**Research Sources:** Hetzner Community, HN "I got hacked" thread, Daniel Tenner guide, OWASP Docker Security

---

## Security Layers Applied

### Layer 1: Hetzner Cloud Firewall (Infrastructure)
- **Firewall ID:** 10551613
- **Rules:**
  - TCP 2222 (SSH) — all sources
  - TCP 4001 (IPFS P2P) — all sources
  - UDP 4001 (IPFS P2P) — all sources
  - ICMP (ping) — all sources
- **Blocks at infrastructure level** before traffic reaches VM

### Layer 2: UFW (Host Firewall)
- Default deny incoming
- Allow: 2222/tcp (SSH), 4001/tcp+udp (IPFS)
- IPFS API (5001) and Gateway (8080) **NOT exposed** (localhost only)

### Layer 3: SSH Hardening
| Setting | Value |
|---------|-------|
| Port | 2222 (non-standard) |
| PermitRootLogin | no |
| PasswordAuthentication | no |
| PubkeyAuthentication | yes |
| AllowUsers | solvr |
| MaxAuthTries | 3 |
| ClientAliveInterval | 300 |
| AllowTcpForwarding | no |
| AllowAgentForwarding | no |
| PermitTunnel | no |
| X11Forwarding | no |

### Layer 4: Fail2Ban
- SSH jail active on port 2222
- 24h ban after 3 failed attempts
- Monitors `/var/log/auth.log`

### Layer 5: Docker Hardening
```json
{
  "log-driver": "json-file",
  "log-opts": {"max-size": "10m", "max-file": "3"},
  "userland-proxy": false,
  "no-new-privileges": true,
  "live-restore": true,
  "default-address-pools": [{"base": "172.17.0.0/16", "size": 24}]
}
```
- **userland-proxy: false** — Prevents Docker proxy security issues
- **no-new-privileges: true** — Containers can't escalate privileges
- **Log limits** — Prevents disk fill from runaway logs
- **Fixed IP pool** — Prevents IP conflicts

### Layer 6: Kernel Hardening (sysctl)
- IP spoofing protection
- SYN flood protection (tcp_syncookies)
- Source routing disabled
- ICMP broadcast ignored
- Martian packet logging
- IPv6 disabled

### Layer 7: System Hardening
- Automatic security updates (unattended-upgrades)
- Audit logging (auditd)
- Secure file permissions
- Non-root user with sudo (solvr)

---

## ⚠️ Known Docker/Firewall Gotcha

**Docker can bypass UFW!** Docker modifies iptables directly.

Our mitigations:
1. IPFS API/Gateway bound to `127.0.0.1` only in docker-compose
2. Hetzner Cloud Firewall as first defense layer
3. Only P2P port (4001) exposed

---

## Verification Commands

```bash
# Test SSH
ssh -i ~/.ssh/solvr_solvr-ipfs-01 -p 2222 solvr@65.109.134.87

# Check firewall status
sudo ufw status
hcloud firewall describe solvr-ipfs-firewall

# Check fail2ban
sudo fail2ban-client status sshd

# Check Docker
sudo docker info | grep -E "Security|Privileges"

# Check listening ports
sudo ss -tlnp
```

---

## What's NOT Exposed

| Service | Port | Status |
|---------|------|--------|
| SSH | 22 | ❌ Blocked |
| IPFS API | 5001 | 🔒 localhost only |
| IPFS Gateway | 8080 | 🔒 localhost only |

---

## Recommendations for Production

1. **Restrict SSH by IP** — Add your home/office IPs to Cloud Firewall
2. **Enable 2FA** — On Hetzner account
3. **Backup snapshots** — Schedule automated Hetzner snapshots
4. **Monitoring** — Add uptime monitoring (UptimeRobot, etc.)
5. **Log aggregation** — Ship logs to external service

---

*Hardened by Claudius 🏴‍☠️*
