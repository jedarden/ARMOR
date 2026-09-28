#!/bin/sh
# Image contract gate: what an untargeted `docker build -f Dockerfile`
# publishes is the LAST stage, and that default image must be the ARMOR
# server — ENTRYPOINT ["/armor"] in exec form (a shell-form entrypoint
# cannot execute on FROM scratch) and no CMD. The companion images are
# published from the same file via explicit --target stages, so those stage
# names must stay addressable with their own entrypoints:
#
#   restore-verifier-runtime  ENTRYPOINT ["/restore-verifier"]
#   armor-fleet-runtime       ENTRYPOINT ["/armor-fleet"]
#
# Why this is a gate and not a comment: images 0.1.1833–0.1.1870 shipped
# /restore-verifier as the entrypoint of ronaldraygun/armor because the
# restore-verifier runtime stage sat last in a multi-stage refactor, and
# the deployed pods crash-looped with restore-verifier's credential error.
# Stage ORDER and target NAMES are exactly the kind of contract a refactor
# breaks silently. Dockerfile.test is held to the same default-image rule
# (it builds ronaldraygun/armor-test); it has no companion stages.
#
#   exit 0  every Dockerfile's last stage is the armor server and both
#           companion --target stages are present with their entrypoints
#   exit 1  contract violation (each hit on stderr)
#   exit 2  structural break: a missing Dockerfile, or one with no stages
#
# The final stage's NAME is deliberately not pinned — renaming it changes
# nothing an untargeted build publishes; only the entrypoint/command
# contract and the companion --target names are. Renaming a companion, or
# giving the server a CMD, is a contract change: make it and update this
# gate (and its suite) in the same change.
#
# Wired into scripts/definition-of-done.sh, scripts/release-gate.sh (so the
# Dockerfile builder stage refuses to produce an image whose own contract
# is broken) and `make docker`. POSIX sh + awk: it runs inside
# golang:alpine build stages.
set -eu

cd "$(cd "$(dirname "$0")/.." && pwd)"

violations=0

# check_file FILE ENFORCE_COMPANIONS — parse FILE's stages and check the
# contract. ENFORCE_COMPANIONS is 1 for Dockerfile (the companion images
# are built from it) and 0 for Dockerfile.test. Status: 1 contract
# violation, 2 structural break.
check_file() {
	file="$1"
	comp="$2"
	if [ ! -f "$file" ]; then
		echo "FAIL: $file not found; the image contract gate covers both Dockerfiles" >&2
		exit 2
	fi
	out="$(awk -v file="$file" -v comp="$comp" '
	# Join backslash-continued lines before parsing, so a split
	# ENTRYPOINT or FROM is still seen as one instruction.
	{
		line = $0
		while (line ~ /\\$/) {
			sub(/\\$/, "", line)
			if ((getline nxt) <= 0) break
			line = line nxt
		}
		sub(/^[ \t]+/, "", line)
		sub(/[ \t]+$/, "", line)
		if (line == "" || line ~ /^#/) next
		n = split(line, f, /[ \t]+/)
		directive = toupper(f[1])
		if (directive == "FROM") {
			nstages++
			cur = nstages
			imgidx = 0
			for (i = 2; i <= n; i++) {
				if (f[i] ~ /^--/) continue
				imgidx = i
				break
			}
			name = ""
			if (imgidx > 0) {
				for (i = imgidx + 1; i < n; i++) {
					if (toupper(f[i]) == "AS") { name = tolower(f[i + 1]); break }
				}
			}
			stagename[cur] = (name == "" ? "<unnamed>" : name)
			next
		}
		if (cur == 0) next
		if (directive == "ENTRYPOINT") {
			rest = ""
			for (i = 2; i <= n; i++) rest = rest (i > 2 ? " " : "") f[i]
			entry[cur] = rest
		} else if (directive == "CMD") {
			cmd[cur] = 1
		}
	}
	END {
		if (nstages == 0) {
			printf "%s: no FROM stages to inspect\n", file
			exit 2
		}
		final = nstages
		fname = stagename[final]
		want = "[\"/armor\"]"
		bad = 0
		if (!(final in entry)) {
			printf "%s: final stage %s declares no ENTRYPOINT; the default image an untargeted build publishes must be the armor server — expected ENTRYPOINT %s\n", file, fname, want
			bad = 1
		} else {
			nrm = entry[final]
			gsub(/[ \t]/, "", nrm)
			if (nrm != want) {
				printf "%s: final stage %s ENTRYPOINT is \"%s\"; the default image an untargeted build publishes must be the armor server — expected ENTRYPOINT %s (exec form, no arguments)\n", file, fname, entry[final], want
				bad = 1
			}
		}
		if (final in cmd) {
			printf "%s: final stage %s declares a CMD; the published server contract is ENTRYPOINT %s with no CMD\n", file, fname, want
			bad = 1
		}
		if (comp == 1) {
			nc = split("restore-verifier-runtime:/restore-verifier armor-fleet-runtime:/armor-fleet", clist, " ")
			for (ci = 1; ci <= nc; ci++) {
				split(clist[ci], pair, ":")
				cname = pair[1]
				cwant = "[\"" pair[2] "\"]"
				found = 0
				for (s = 1; s <= nstages; s++) {
					if (stagename[s] == cname) { found = s; break }
				}
				if (found == 0) {
					printf "%s: companion --target stage %s is gone; the %s image is published from it (docker build --target %s)\n", file, cname, pair[2], cname
					bad = 1
					continue
				}
				if (found == final) {
					printf "%s: companion stage %s is the FINAL stage — an untargeted build would publish %s as ronaldraygun/armor\n", file, cname, pair[2]
					bad = 1
				}
				if (!(found in entry)) {
					printf "%s: companion stage %s declares no ENTRYPOINT; expected ENTRYPOINT %s\n", file, cname, cwant
					bad = 1
				} else {
					nrm = entry[found]
					gsub(/[ \t]/, "", nrm)
					if (nrm != cwant) {
						printf "%s: companion stage %s ENTRYPOINT is \"%s\"; expected %s\n", file, cname, entry[found], cwant
						bad = 1
					}
				}
			}
		}
		if (bad) exit 1
	}
	' "$file")" && status=0 || status=$?
	if [ "$status" -eq 2 ]; then
		echo "FAIL: $out" >&2
		exit 2
	fi
	if [ -n "$out" ]; then
		printf '%s\n' "$out" >&2
		violations=1
	fi
}

check_file Dockerfile 1
check_file Dockerfile.test 0

if [ "$violations" -ne 0 ]; then
	echo "FAIL: Dockerfile image contract broken — the default (untargeted) image must stay the armor server and the companion --target stages addressable (scripts/image-contract-gate.sh; images 0.1.1833-0.1.1870 shipped /restore-verifier as ronaldraygun/armor)" >&2
	exit 1
fi
echo "Image contract: default image of Dockerfile and Dockerfile.test is the armor server (ENTRYPOINT [\"/armor\"], no CMD); --target restore-verifier-runtime and --target armor-fleet-runtime addressable"
