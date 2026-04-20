#!/usr/bin/env python3
import sys, io
sys.stdout = io.TextIOWrapper(sys.stdout.buffer, encoding='utf-8', errors='replace')
sys.stderr = io.TextIOWrapper(sys.stderr.buffer, encoding='utf-8', errors='replace')
"""
Bulk WhatsApp Message Sender
============================
Sends personalized messages to contacts listed in contacts.csv
using a message_template.txt with {variable} placeholders.

Usage:
    python bulk_send.py
    python bulk_send.py --dry-run          # Preview messages without sending
    python bulk_send.py --delay-min 5 --delay-max 15   # Custom delays (seconds)
    python bulk_send.py --start-from 3     # Skip first N contacts (resume)
"""

import csv
import time
import random
import json
import argparse
import logging
import sys
import requests
from datetime import datetime
from pathlib import Path

# ─────────────────────────────────────────────
# CONFIG
# ─────────────────────────────────────────────
SERVER_URL   = "http://localhost:8080/api/send"   # Go bridge endpoint
CSV_FILE     = "contacts.csv"                      # Contacts file
TEMPLATE_FILE = "message_template.txt"             # Message template
LOG_FILE     = "bulk_send_log.txt"                 # Log file
DELAY_MIN    = 8    # Minimum seconds between messages
DELAY_MAX    = 20   # Maximum seconds between messages
# ─────────────────────────────────────────────

# Setup logging (to file + console)
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s | %(levelname)s | %(message)s",
    datefmt="%Y-%m-%d %H:%M:%S",
    handlers=[
        logging.FileHandler(LOG_FILE, encoding="utf-8"),
        logging.StreamHandler(sys.stdout),
    ]
)
log = logging.getLogger(__name__)


def load_template(path: str) -> str:
    """Load the message template from file."""
    template_path = Path(path)
    if not template_path.exists():
        log.error(f"Template file not found: {path}")
        sys.exit(1)
    return template_path.read_text(encoding="utf-8").strip()


def load_contacts(path: str) -> list[dict]:
    """Load contacts from CSV file."""
    contacts_path = Path(path)
    if not contacts_path.exists():
        log.error(f"Contacts file not found: {path}")
        sys.exit(1)
    with open(contacts_path, newline="", encoding="utf-8") as f:
        reader = csv.DictReader(f)
        contacts = [row for row in reader if row.get("phone", "").strip()]
    log.info(f"Loaded {len(contacts)} contacts from {path}")
    return contacts


def build_message(template: str, row: dict) -> str:
    """Substitute {variable} placeholders with CSV column values."""
    try:
        return template.format_map(row)
    except KeyError as e:
        log.warning(f"Missing variable {e} for contact {row.get('name', '?')} — skipping substitution")
        return template


def normalize_phone(phone: str) -> str:
    """Ensure phone is in JID format: countrycode+number@s.whatsapp.net"""
    phone = phone.strip().replace("+", "").replace(" ", "").replace("-", "")
    if "@" not in phone:
        phone = f"{phone}@s.whatsapp.net"
    return phone


def send_message(jid: str, message: str, dry_run: bool = False) -> bool:
    """Send a single message via the Go bridge HTTP API."""
    if dry_run:
        return True  # Simulate success

    payload = {"jid": jid, "message": message}
    try:
        resp = requests.post(SERVER_URL, json=payload, timeout=15)
        data = resp.json()
        if data.get("success"):
            return True
        else:
            log.error(f"API Error: {data.get('error', 'Unknown error')}")
            return False
    except requests.exceptions.ConnectionError:
        log.error("❌ Cannot connect to Go bridge at localhost:8080 — is it running?")
        return False
    except Exception as e:
        log.error(f"Request failed: {e}")
        return False


def print_banner():
    print("\n" + "=" * 55)
    print("  [WA]  WhatsApp Bulk Sender - Antigravity")
    print("=" * 55 + "\n")


def main():
    parser = argparse.ArgumentParser(description="Bulk WhatsApp message sender")
    parser.add_argument("--dry-run",    action="store_true", help="Preview messages without sending")
    parser.add_argument("--delay-min",  type=float, default=DELAY_MIN,  help="Min delay between sends (seconds)")
    parser.add_argument("--delay-max",  type=float, default=DELAY_MAX,  help="Max delay between sends (seconds)")
    parser.add_argument("--start-from", type=int,   default=0,          help="Skip first N contacts (resume mode)")
    parser.add_argument("--csv",        type=str,   default=CSV_FILE,   help="Path to contacts CSV")
    parser.add_argument("--template",   type=str,   default=TEMPLATE_FILE, help="Path to message template")
    args = parser.parse_args()

    print_banner()

    if args.dry_run:
        log.info("🔍 DRY RUN MODE — No messages will actually be sent\n")

    template  = load_template(args.template)
    contacts  = load_contacts(args.csv)

    total     = len(contacts)
    sent      = 0
    failed    = 0
    skipped   = args.start_from

    if skipped > 0:
        log.info(f"⏩ Skipping first {skipped} contacts (resume mode)\n")
        contacts = contacts[skipped:]

    log.info(f"📋 Starting bulk send: {len(contacts)} contacts | delay {args.delay_min}–{args.delay_max}s\n")

    for i, row in enumerate(contacts, start=skipped + 1):
        name    = row.get("name", "Contact").strip()
        phone   = normalize_phone(row.get("phone", ""))
        message = build_message(template, row)

        log.info(f"[{i}/{total}] 👤 {name} → {phone}")

        if args.dry_run:
            print(f"\n{'─'*50}")
            print(f"To: {name} ({phone})")
            print(f"Message:\n{message}")
            print("-" * 50 + "\n")
            sent += 1
        else:
            success = send_message(phone, message, dry_run=False)
            if success:
                log.info(f"  ✅ Sent successfully")
                sent += 1
            else:
                log.warning(f"  ❌ Failed to send")
                failed += 1

        # Randomized delay to avoid rate-limiting (skip delay after last contact)
        if i < total:
            delay = random.uniform(args.delay_min, args.delay_max)
            log.info(f"  ⏳ Waiting {delay:.1f}s before next message...")
            time.sleep(delay)

    # ── Summary ──────────────────────────────────
    print("\n" + "=" * 55)
    log.info(f"DONE -- Sent: {sent} | Failed: {failed} | Total: {total}")
    print("=" * 55 + "\n")

    if failed > 0:
        log.warning(f"⚠️  {failed} message(s) failed. Check {LOG_FILE} for details.")


if __name__ == "__main__":
    main()
