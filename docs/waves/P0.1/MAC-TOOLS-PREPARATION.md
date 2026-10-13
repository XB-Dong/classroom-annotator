# Mac preparation while the final media Agent is under review

The user has authorized P0.1. This task prepares dependencies only; it does not activate an unreviewed Agent or tunnel.

1. Preserve the existing authorized source service at `127.0.0.1:19090`. Do not change its configuration, port, whitelist, tokens or files. Do not transfer any video, credentials or private keys to GitHub or GLM.
2. Verify the known absolute FFmpeg binary with `-version`. It is outside PATH; keep it available and record its actual path/version.
3. Locate a usable native arm64 ffprobe and verify `-version`. If Homebrew is already installed, install its official ffmpeg formula if needed to obtain ffprobe; keep the existing source service unchanged. Resolve the resulting binary's absolute path. Do not install Go, Node, Docker or PostgreSQL for the new prebuilt Agent. If installation is blocked, report the safe error and stop dependency installation after two attempts.
4. Reserve a new runtime directory outside Git for the future Agent, private token files and immutable media. Do not use the existing source directory as the writable Agent root. Do not start the old draft `4efd3e5` or first repair `b39d408`; a final reviewed private package will be provided separately.
5. Return only: timestamp with timezone, arm64 verification, FFmpeg and ffprobe absolute paths and actual versions, original source service status, and any sanitized blocker. Do not print passwords, tokens, company API keys, SSH private keys or media bytes.
