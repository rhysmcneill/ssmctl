# ssmctl demo tape — generates .github/demo.gif
#
# Requirements:
#   brew install charmbracelet/tap/vhs
#
# Usage:
#   1. Replace "web-1" below with a real instance Name tag or ID (i-...)
#      that is Online in your AWS account.
#   2. Ensure your AWS credentials are configured and the SSM Agent is
#      running on the target instance.
#   3. Run:
#        vhs .github/demo.tape
#   4. Commit the generated .github/demo.gif

Output .github/demo.gif

Set Shell "bash"
Set FontSize 14
Set FontFamily "JetBrains Mono"
Set Width 1100
Set Height 550
Set Theme "Catppuccin Mocha"
Set Padding 24
Set PlaybackSpeed 1.0
Set TypingSpeed 60ms

# ─── List instances ──────────────────────────────────────────────────────────
Sleep 500ms
Type "ssmctl list"
Sleep 300ms
Enter
Sleep 3s

# ─── Run a one-shot command ───────────────────────────────────────────────────
Sleep 800ms
Type "ssmctl run web-1 -- uptime"
Sleep 300ms
Enter
Sleep 4s

# ─── Copy a file up ───────────────────────────────────────────────────────────
Sleep 800ms
Type "ssmctl cp ./nginx.conf web-1:/etc/nginx/nginx.conf"
Sleep 300ms
Enter
Sleep 5s

# ─── Port-forward to RDS ─────────────────────────────────────────────────────
# Replace the RDS hostname with your own
Sleep 800ms
Type "ssmctl forward web-1 --local 5432 --remote prod-db.cluster-xyz.rds.amazonaws.com:5432"
Sleep 300ms
Enter
Sleep 3s

# Show the tunnel is live then exit cleanly
Ctrl+C
Sleep 1s
