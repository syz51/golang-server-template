# Releasing the template

Template releases are snapshots consumed by `gonew`; they are not server deployments. Release tags must use canonical Go module versions such as `v1.2.3` or `v1.2.3-rc.1`, without build metadata, with the release's major version matching [Go module-path rules](https://go.dev/ref/mod#major-version-suffixes). The current module path has no `/v2` suffix, so release only v0 or v1 tags until a major-version migration is implemented.

## Validate a candidate

1. Review the complete diff and ensure the canonical module path and internal imports match the public repository.
2. Run `go run ./tools/releasecheck -tag v1.0.0-rc.1` (substituting the candidate tag) and the checks in [CONTRIBUTING.md](CONTRIBUTING.md). The generation check uses the working tree, so it can validate changes before a commit exists.
3. Verify the Linux and macOS CI jobs and the Linux container job on the exact candidate commit. Review vulnerability output, including reachability, and scan the chosen runtime image according to your publishing environment's policy.
4. Confirm README commands, supported Go versions, tool pins, `.env.example`, YAML defaults, and license notices match the implementation. Summarize changes and migration notes for generated projects, including configuration/API changes.
5. Confirm private vulnerability reporting is enabled in repository settings, or provide a private reporting channel in `SECURITY.md`. This setting cannot be established by committing this file.

## Publish with maintainer authorization

Only a maintainer authorized to publish should create and push a version tag on the reviewed commit. Never move an existing published tag. The tag-triggered validation workflow verifies the tagged source but does not itself authorize publication or deploy an image. Wait for all tag checks to pass before announcing a release.

After publication, run the pinned `gonew` tool against the exact new public tag into a fresh directory. Run its tests and start the generated server, confirming `/health` and a create/read/update/delete cycle. This public-module check is distinct from the local proxy check and catches publication or module-proxy problems.

Publish release notes with the exact tag, supported toolchain/platforms, verified checks, and any remaining limitations. Recommend the exact tag to users who need repeatable generation. Retain upstream MIT notices in source and distributed images.

No publishing credentials or registry tokens are required for local acceptance checks. CI performs validation only; creating a GitHub release, pushing tags, or publishing container images is a separate authorized maintainer action.
