# ARMOR 60-second demo recording — the README's demo, end to end:
# up, put, stored-bytes-are-ciphertext check, get, cmp, down.
#
# Run from the repository root:  vhs docs/images/armor-demo.tape
# The demo runs inside an isolated temporary directory; nothing is written
# into the checkout, and the GIF is written next to this tape.
Output "docs/images/armor-demo.gif"
Require docker

Set Shell "bash"
Set FontSize 15
Set Width 1000
Set Height 480
Set TypingSpeed 30ms

# Set up the isolated demo directory, the demo object, and the aws() helper
# from the README without spending GIF time on boilerplate.
Hide
Type 'd=$(mktemp -d) && cp compose.yaml "$d" && cd "$d" && echo "hello, ARMOR" > demo.txt'
Enter
Type 'aws() { docker run --rm --network container:"$(docker compose --profile demo ps -q armor-demo)" -v "$PWD:/work" -w /work -e AWS_ACCESS_KEY_ID=armor -e AWS_SECRET_ACCESS_KEY=armor-demo-secret -e AWS_DEFAULT_REGION=us-east-1 amazon/aws-cli:2.29.0 --endpoint-url http://127.0.0.1:9000 s3 "$@"; }'
Enter
Show

Type "docker compose --profile demo up -d armor-demo"
Enter
Sleep 2s
Hide
Type 'curl --fail --retry 30 --retry-connrefused --retry-delay 1 -so /dev/null http://127.0.0.1:9001/healthz'
Enter
Show

Type "aws ls"
Enter
Sleep 3s

Type "aws cp demo.txt s3://demo-bucket/"
Enter
Sleep 3s

Type 'docker compose --profile demo cp armor-demo:/tmp ./demo-storage'
Enter
Type 'find ./demo-storage -type f -name demo.txt'
Enter
Sleep 1s
Type 'grep -rq "hello, ARMOR" ./demo-storage || echo "stored bytes are ciphertext"'
Enter
Sleep 1s

Type 'aws cp s3://demo-bucket/demo.txt demo-out.txt'
Enter
Sleep 3s
Type 'cmp demo.txt demo-out.txt && echo "round trip: byte-identical"'
Enter
Sleep 1s

Type 'docker compose --profile demo down'
Enter
Sleep 3s

Hide
Type 'cd / && rm -rf "$d"'
Enter
Show
