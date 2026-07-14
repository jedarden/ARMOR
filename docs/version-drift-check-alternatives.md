# ARMOR Version Drift Check - Scheduling Options

## Note on Current System

This system (NixOS) may not have traditional cron available. Here are alternative scheduling approaches:

## Option 1: systemd Timer (Recommended for NixOS)

Create a systemd user service and timer:

```bash
# Create the service file
~/.config/systemd/user/armor-version-drift-check.service
```

```ini
[Unit]
Description=ARMOR Version Drift Check
After=network.target

[Service]
Type=oneshot
WorkingDirectory=/home/coding/ARMOR
ExecStart=/home/coding/ARMOR/scripts/check-armor-version-drift.py
StandardOutput=append:/home/coding/ARMOR/logs/version-drift-check.log
StandardError=append:/home/coding/ARMOR/logs/version-drift-check.log
```

```bash
# Create the timer file
~/.config/systemd/user/armor-version-drift-check.timer
```

```ini
[Unit]
Description=ARMOR Version Drift Check (Daily)
Requires=armor-version-drift-check.service

[Timer]
OnCalendar=daily
OnCalendar=09:17
RandomizedDelaySec=600

[Install]
WantedBy=timers.target
```

Enable the timer:
```bash
systemctl --user daemon-reload
systemctl --user enable armor-version-drift-check.timer
systemctl --user start armor-version-drift-check.timer
```

## Option 2: Claude Code Loop

Use the Claude Code /loop skill:

```
/loop 1d ./scripts/check-armor-version-drift.py
```

This runs the check daily within the Claude Code session.

## Option 3: Manual Execution

Run manually when needed:

```bash
./scripts/check-armor-version-drift.py
```

## Option 4: External Cron

If you have access to external cron (via another system), you can use the traditional cron setup from `scripts/armor-version-drift-check.cron`.

## Log Location

All methods write logs to: `/home/coding/ARMOR/logs/version-drift-check.log`

## Testing

Regardless of scheduling method, test the script first:

```bash
./scripts/check-armor-version-drift.py
```
