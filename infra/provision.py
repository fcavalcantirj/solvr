#!/usr/bin/env python3
"""
Solvr Infrastructure Provisioner — Hetzner Cloud
=================================================

Provisions Solvr infrastructure nodes using hcloud-python.

Usage:
    python provision.py --name solvr-ipfs-01 --type cx32 --purpose ipfs
    python provision.py --name solvr-api-01 --type cx32 --purpose api
    python provision.py --list
    python provision.py --destroy solvr-ipfs-01

Environment:
    HCLOUD_TOKEN — Hetzner Cloud API token (required)

Follows patterns from openclaw-deploy but uses hcloud-python for cleaner code.
"""

import argparse
import json
import os
import subprocess
import sys
import time
from datetime import datetime, timezone
from pathlib import Path

try:
    from hcloud import Client
    from hcloud.images import Image
    from hcloud.locations import Location
    from hcloud.server_types import ServerType
    from hcloud.ssh_keys import SSHKey
except ImportError:
    print("Missing hcloud library. Install with: pip install hcloud")
    sys.exit(1)


# =============================================================================
# Configuration
# =============================================================================

INFRA_DIR = Path(__file__).parent
INSTANCES_DIR = INFRA_DIR / "instances"
SSH_DIR = Path.home() / ".ssh"

# Server type presets by purpose
SERVER_PRESETS = {
    "ipfs": {
        "type": "cx33",      # 4 vCPU, 8GB RAM, 80GB SSD — €5.49/mo
        "image": "ubuntu-24.04",
        "labels": {"service": "ipfs", "component": "kubo"},
    },
    "api": {
        "type": "cx33",      # Can scale independently
        "image": "ubuntu-24.04", 
        "labels": {"service": "api", "component": "solvr"},
    },
    "cluster": {
        "type": "cx43",      # 8 vCPU, 16GB RAM — for IPFS cluster nodes
        "image": "ubuntu-24.04",
        "labels": {"service": "ipfs", "component": "cluster"},
    },
}

DEFAULT_LOCATION = "nbg1"  # Nuremberg, Germany (cheapest)
LOCATIONS = ["nbg1", "fsn1", "hel1", "ash", "hil"]  # EU + US


# =============================================================================
# Utilities
# =============================================================================

def get_client() -> Client:
    """Get authenticated Hetzner Cloud client."""
    token = os.environ.get("HCLOUD_TOKEN")
    if not token:
        # Try to get from hcloud CLI config
        try:
            result = subprocess.run(
                ["hcloud", "context", "active"],
                capture_output=True, text=True, check=True
            )
            context_name = result.stdout.strip()
            # hcloud stores tokens in ~/.config/hcloud/cli.toml
            config_path = Path.home() / ".config" / "hcloud" / "cli.toml"
            if config_path.exists():
                import re
                content = config_path.read_text()
                # Parse TOML manually (avoid dependency)
                match = re.search(rf'\[contexts\.{context_name}\].*?token\s*=\s*"([^"]+)"', 
                                 content, re.DOTALL)
                if match:
                    token = match.group(1)
        except Exception:
            pass
    
    if not token:
        print("ERROR: HCLOUD_TOKEN not set and no active hcloud context found")
        print("Set HCLOUD_TOKEN or run: hcloud context create <name>")
        sys.exit(1)
    
    return Client(
        token=token,
        application_name="solvr-infra",
        application_version="1.0.0"
    )


def generate_ssh_key(name: str) -> tuple[Path, str]:
    """Generate SSH key pair for instance."""
    key_path = SSH_DIR / f"solvr_{name}"
    pub_path = key_path.with_suffix(".pub")
    
    if not key_path.exists():
        print(f"🔑 Generating SSH key: {key_path}")
        subprocess.run([
            "ssh-keygen", "-t", "ed25519", 
            "-f", str(key_path), 
            "-N", "",
            "-C", f"solvr-{name}"
        ], check=True)
    else:
        print(f"✓ SSH key exists: {key_path}")
    
    return key_path, pub_path.read_text().strip()


def wait_for_ssh(ip: str, key_path: Path, max_retries: int = 10) -> bool:
    """Wait for SSH to become available."""
    print("⏳ Waiting for SSH...")
    for i in range(max_retries):
        try:
            result = subprocess.run([
                "ssh", "-i", str(key_path),
                "-o", "ConnectTimeout=5",
                "-o", "StrictHostKeyChecking=no",
                "-o", "UserKnownHostsFile=/dev/null",
                f"root@{ip}", "echo ok"
            ], capture_output=True, text=True, timeout=10)
            if result.returncode == 0:
                print("✓ SSH ready")
                return True
        except Exception:
            pass
        print(f"  Retry {i+1}/{max_retries}...")
        time.sleep(5)
    return False


def save_metadata(name: str, data: dict):
    """Save instance metadata to JSON."""
    INSTANCES_DIR.mkdir(parents=True, exist_ok=True)
    instance_dir = INSTANCES_DIR / name
    instance_dir.mkdir(exist_ok=True)
    
    meta_path = instance_dir / "metadata.json"
    meta_path.write_text(json.dumps(data, indent=2))
    print(f"✓ Metadata: {meta_path}")


def load_metadata(name: str) -> dict | None:
    """Load instance metadata."""
    meta_path = INSTANCES_DIR / name / "metadata.json"
    if meta_path.exists():
        return json.loads(meta_path.read_text())
    return None


# =============================================================================
# Cloud-Init for IPFS Node
# =============================================================================

CLOUD_INIT_IPFS = """#cloud-config
package_update: true
package_upgrade: true

packages:
  - docker.io
  - docker-compose
  - jq
  - htop
  - curl

runcmd:
  # Enable Docker
  - systemctl enable docker
  - systemctl start docker
  
  # Create directories
  - mkdir -p /opt/solvr/ipfs
  - mkdir -p /var/lib/ipfs
  
  # Create docker-compose for Kubo
  - |
    cat > /opt/solvr/ipfs/docker-compose.yml << 'COMPOSE'
    version: "3.9"
    services:
      ipfs:
        image: ipfs/kubo:latest
        container_name: solvr-ipfs
        restart: unless-stopped
        environment:
          - IPFS_PROFILE=server
        volumes:
          - /var/lib/ipfs:/data/ipfs
        ports:
          - "4001:4001"           # P2P
          - "127.0.0.1:5001:5001" # API (localhost only)
          - "127.0.0.1:8080:8080" # Gateway (localhost only)
        healthcheck:
          test: ["CMD", "ipfs", "id"]
          interval: 30s
          timeout: 10s
          retries: 3
    COMPOSE
  
  # Start IPFS
  - cd /opt/solvr/ipfs && docker-compose up -d
  
  # Wait for IPFS to initialize
  - sleep 30
  
  # Configure IPFS for API access from Solvr backend
  - docker exec solvr-ipfs ipfs config --json API.HTTPHeaders.Access-Control-Allow-Origin '["*"]'
  - docker exec solvr-ipfs ipfs config --json API.HTTPHeaders.Access-Control-Allow-Methods '["PUT", "POST", "GET"]'
  
  # Restart to apply config
  - docker restart solvr-ipfs
  
  # Create status script
  - |
    cat > /opt/solvr/ipfs/status.sh << 'STATUS'
    #!/bin/bash
    echo "=== IPFS Node Status ==="
    docker exec solvr-ipfs ipfs id | jq -r '.ID'
    echo ""
    echo "=== Repo Stats ==="
    docker exec solvr-ipfs ipfs repo stat
    echo ""
    echo "=== Swarm Peers ==="
    docker exec solvr-ipfs ipfs swarm peers | wc -l
    STATUS
    chmod +x /opt/solvr/ipfs/status.sh

final_message: "Solvr IPFS node ready! Run: /opt/solvr/ipfs/status.sh"
"""


# =============================================================================
# Commands
# =============================================================================

def cmd_provision(args):
    """Provision a new server."""
    client = get_client()
    name = args.name
    purpose = args.purpose or "ipfs"
    location = args.location or DEFAULT_LOCATION
    
    # Get preset or use custom type
    preset = SERVER_PRESETS.get(purpose, SERVER_PRESETS["ipfs"])
    server_type = args.type or preset["type"]
    image = preset["image"]
    labels = preset["labels"].copy()
    labels["name"] = name
    labels["managed-by"] = "solvr-infra"
    
    print("═" * 60)
    print(f"  Provisioning Solvr Node")
    print(f"  Name:     {name}")
    print(f"  Purpose:  {purpose}")
    print(f"  Type:     {server_type}")
    print(f"  Location: {location}")
    print("═" * 60)
    print()
    
    # Check if server already exists
    existing = client.servers.get_by_name(name)
    if existing:
        print(f"✓ Server '{name}' already exists")
        print(f"  IP: {existing.public_net.ipv4.ip}")
        return
    
    # Generate SSH key
    ssh_key_path, ssh_pub_key = generate_ssh_key(name)
    
    # Upload SSH key to Hetzner
    ssh_key_name = f"solvr-{name}"
    existing_key = client.ssh_keys.get_by_name(ssh_key_name)
    if existing_key:
        print(f"✓ SSH key '{ssh_key_name}' exists in Hetzner")
        ssh_key = existing_key
    else:
        print(f"📤 Uploading SSH key to Hetzner...")
        ssh_key = client.ssh_keys.create(name=ssh_key_name, public_key=ssh_pub_key)
    
    # Select cloud-init based on purpose
    user_data = CLOUD_INIT_IPFS if purpose == "ipfs" else None
    
    # Create server
    print(f"🖥️  Creating server '{name}'...")
    response = client.servers.create(
        name=name,
        server_type=ServerType(name=server_type),
        image=Image(name=image),
        location=Location(name=location),
        ssh_keys=[ssh_key],
        labels=labels,
        user_data=user_data,
    )
    
    server = response.server
    ip = server.public_net.ipv4.ip
    print(f"✓ Server created: {ip}")
    
    # Wait for SSH
    print("⏳ Waiting 45s for boot + cloud-init...")
    time.sleep(45)
    
    if not wait_for_ssh(ip, ssh_key_path):
        print("⚠️  SSH not ready yet, but server is running")
    
    # Save metadata
    metadata = {
        "name": name,
        "purpose": purpose,
        "ip": ip,
        "server_type": server_type,
        "location": location,
        "labels": labels,
        "ssh_key_path": str(ssh_key_path),
        "created_at": datetime.now(timezone.utc).isoformat(),
        "hetzner_id": server.id,
    }
    save_metadata(name, metadata)
    
    print()
    print("═" * 60)
    print("  ✅ Provisioning Complete")
    print()
    print(f"  Server:  {name}")
    print(f"  IP:      {ip}")
    print(f"  SSH:     ssh -i {ssh_key_path} root@{ip}")
    print()
    if purpose == "ipfs":
        print("  IPFS Status: /opt/solvr/ipfs/status.sh")
        print("  IPFS API:    http://127.0.0.1:5001 (via SSH tunnel)")
    print("═" * 60)


def cmd_list(args):
    """List all Solvr servers."""
    client = get_client()
    
    servers = client.servers.get_all(label_selector="managed-by=solvr-infra")
    
    if not servers:
        print("No Solvr servers found")
        return
    
    print(f"{'NAME':<20} {'IP':<16} {'TYPE':<8} {'STATUS':<10} {'PURPOSE':<10}")
    print("-" * 70)
    for s in servers:
        purpose = s.labels.get("service", "unknown")
        print(f"{s.name:<20} {s.public_net.ipv4.ip:<16} {s.server_type.name:<8} {s.status:<10} {purpose:<10}")


def cmd_destroy(args):
    """Destroy a server."""
    client = get_client()
    name = args.name
    
    server = client.servers.get_by_name(name)
    if not server:
        print(f"Server '{name}' not found")
        return
    
    if not args.yes:
        confirm = input(f"⚠️  Destroy server '{name}' ({server.public_net.ipv4.ip})? [y/N] ")
        if confirm.lower() != 'y':
            print("Cancelled")
            return
    
    print(f"🗑️  Destroying server '{name}'...")
    client.servers.delete(server)
    
    # Clean up local metadata
    instance_dir = INSTANCES_DIR / name
    if instance_dir.exists():
        import shutil
        shutil.rmtree(instance_dir)
    
    # Optionally remove SSH key from Hetzner
    ssh_key = client.ssh_keys.get_by_name(f"solvr-{name}")
    if ssh_key:
        client.ssh_keys.delete(ssh_key)
    
    print(f"✓ Server '{name}' destroyed")


def cmd_status(args):
    """Show status of a specific server."""
    client = get_client()
    name = args.name
    
    server = client.servers.get_by_name(name)
    if not server:
        print(f"Server '{name}' not found")
        return
    
    meta = load_metadata(name)
    
    print("═" * 60)
    print(f"  Server: {name}")
    print("═" * 60)
    print(f"  IP:       {server.public_net.ipv4.ip}")
    print(f"  Status:   {server.status}")
    print(f"  Type:     {server.server_type.name}")
    print(f"  Location: {server.datacenter.name}")
    print(f"  Labels:   {server.labels}")
    if meta:
        print(f"  Created:  {meta.get('created_at', 'unknown')}")
        print(f"  SSH:      ssh -i {meta.get('ssh_key_path')} root@{server.public_net.ipv4.ip}")
    print("═" * 60)


# =============================================================================
# Main
# =============================================================================

def main():
    parser = argparse.ArgumentParser(
        description="Solvr Infrastructure Provisioner",
        formatter_class=argparse.RawDescriptionHelpFormatter,
        epilog="""
Examples:
  # Provision IPFS node
  python provision.py --name solvr-ipfs-01 --purpose ipfs
  
  # Provision API node  
  python provision.py --name solvr-api-01 --purpose api --type cx42
  
  # List all servers
  python provision.py --list
  
  # Get server status
  python provision.py --status solvr-ipfs-01
  
  # Destroy server
  python provision.py --destroy solvr-ipfs-01
"""
    )
    
    parser.add_argument("--name", help="Server name")
    parser.add_argument("--purpose", choices=["ipfs", "api", "cluster"], 
                       help="Server purpose (sets defaults)")
    parser.add_argument("--type", help="Server type (e.g., cx32, cx42)")
    parser.add_argument("--location", choices=LOCATIONS, help="Datacenter location")
    parser.add_argument("--list", action="store_true", help="List all servers")
    parser.add_argument("--status", metavar="NAME", help="Show server status")
    parser.add_argument("--destroy", metavar="NAME", help="Destroy a server")
    parser.add_argument("--yes", "-y", action="store_true", help="Skip confirmation")
    
    args = parser.parse_args()
    
    if args.list:
        cmd_list(args)
    elif args.status:
        args.name = args.status
        cmd_status(args)
    elif args.destroy:
        args.name = args.destroy
        cmd_destroy(args)
    elif args.name:
        cmd_provision(args)
    else:
        parser.print_help()


if __name__ == "__main__":
    main()
