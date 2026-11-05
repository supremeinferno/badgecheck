# badgecheck

A small command-line tool that uses the GitHub GraphQL API to show how close a user is to the GitHub achievements that can be counted from public data.

```text
$ badgecheck octocat
Galaxy Brain  3 accepted answers   next tier at 8
Pull Shark    12 merged PRs        next tier at 16
Starstruck    6 stars on top repo  needs 16
```

## Install

Needs Go 1.22 or newer.

```sh
go install github.com/supremeinferno/badgecheck@latest
```

## Usage

```sh
badgecheck <github-username>
```

GitHub's GraphQL API requires a token. `badgecheck` reads `GITHUB_TOKEN`, then falls back to `gh auth token` if you use the GitHub CLI. It only reads data, so a token with no extra scopes is enough for public information.

## What it counts

| Achievement | Count | Tiers |
|---|---|---|
| Galaxy Brain | Discussion replies marked as the answer | 2, 8, 16, 32 |
| Pull Shark | Pull requests authored and merged | 2, 16, 128, 1024 |
| Starstruck | Stars on the user's most-starred non-fork repository | 16, 128, 512, 4096 |

## Limitations

- GitHub doesn't publish the achievement rules. The tiers above are the numbers commonly reported by users, and they may change.
- This shows progress, not awarded badges. A count that meets a tier doesn't prove GitHub has awarded the badge, and GitHub's own count may differ. For example, a token can see private work that doesn't count.
- Achievements such as Pair Extraordinaire, Quickdraw and YOLO aren't covered.

## Support

Open an [issue](https://github.com/supremeinferno/badgecheck/issues).

## License

[MIT](LICENSE)
