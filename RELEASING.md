# Releasing the template

Template releases are snapshots consumed by `gonew`; they are not server deployments. Release tags must use canonical Go module versions such as `v1.2.3` or `v1.2.3-rc.1`, without build metadata, with the release's major version matching [Go module-path rules](https://go.dev/ref/mod#major-version-suffixes). The current module path has no `/v2` suffix, so release only v0 or v1 tags until a major-version migration is implemented.

## Validate a candidate

1. Review the complete diff and ensure the canonical module path and internal imports match the public repository. Run the local checks in [CONTRIBUTING.md](CONTRIBUTING.md); generation checks include relevant uncommitted files.
2. Confirm README commands, supported Go versions, tool pins, `.env.example`, YAML defaults, and license notices match the implementation. Include migration notes for generated projects when configuration or APIs change. Retain upstream MIT notices in source and distributed images.
3. Check private vulnerability reporting in repository settings, or provide a private reporting channel in `SECURITY.md`. Do not assume reporting is enabled; committing these files cannot enable it.
4. Commit and push the candidate, then wait for **Delivery validation** on that exact commit. Linux and macOS native checks, generated-consumer checks, Linux production container checks, and vulnerability scanning must pass. Review advisory reachability and scan the chosen runtime image according to your publishing environment's policy.
5. Set the intended tag and validate its module-major compatibility:

   ```sh
   RELEASE_TAG=v0.1.0 # example only; choose an unused version
   CANDIDATE_COMMIT="$(git rev-parse HEAD)"
   go run ./tools/releasecheck -tag "$RELEASE_TAG"
   ```

For optional public verification before tagging, run `go run ./tools/publiccheck -version "$CANDIDATE_COMMIT"` after pushing the commit. This needs public repository access, network access to the Go module proxy/checksum database, and a running Linux-container Docker daemon. It fails when Docker is unavailable. This checks the published commit; the tag workflow still verifies the exact version tag separately.

## Publish with maintainer authorization

Only a maintainer authorized to publish should create and push an annotated version tag on the reviewed candidate commit:

```sh
git tag -a "$RELEASE_TAG" "$CANDIDATE_COMMIT" -m "Release $RELEASE_TAG"
git push origin "refs/tags/$RELEASE_TAG"
```

Pushing the tag triggers all validation jobs again. Only a tag **push** can run the final `publish` job, after `release-tag`, `native`, `generated-consumer`, `container`, and `vulnerabilities` succeed. The job checks out the tag-push event commit (`github.sha`) and verifies that the remote tag still peels to that commit before running:

```sh
go run ./tools/publiccheck -version "$RELEASE_TAG" -commit "$CANDIDATE_COMMIT"
```

Here `CANDIDATE_COMMIT` is the full event commit SHA. This verifies the exact tag through the public Go module proxy and checksum database, requires the downloaded module origin to match that commit, then checks a generated consumer's race tests, vet, native HTTP behavior, SIGTERM shutdown, and production container. Network access and Docker are mandatory. Public verification is distinct from the working-tree generation check and runs before GitHub Release creation, including on retries. Immediately before any release API operation, CI fetches and compares the remote tag again to detect movement during public tests. These checks bind the event commit, checked-out source, and public module origin; tags must remain immutable because the comparisons cannot atomically exclude every later tag change.

After those checks pass, CI creates a GitHub Release with the tag as its title and generated change notes plus toolchain/platform and consumer-verification notes. Prerelease tags are marked as prereleases. Automation leaves the GitHub **Latest** designation unchanged for all versions, so an older maintenance release cannot displace a newer release; maintainers may choose that designation separately. The workflow uses the current repository identity, so generated projects can reuse it with their own public module/repository and release policy. It publishes no binaries or container images.

Observe **Delivery validation** in the repository's Actions tab and open the release link in the `publish` job summary. Wait for successful public verification and release creation before announcing availability. Recommend the exact verified published tag for repeatable generation; before a tag exists, pin a verified full commit hash. Never describe an unpublished or unverified version as available.

## Recovery

Validation failures prevent creation of a GitHub Release. A release API/network error may occur after GitHub accepted creation; inspect the release and rerun the failed workflow without moving tags. Retries revalidate the public consumer, preserve an existing published release without modifying it, and fail visibly if an existing draft needs maintainer attention. Publication is serialized per tag without cancelling an in-progress run.

If either remote-tag comparison fails, publication stops. Investigate the tag movement and publish the intended source under a new unused version; do not retarget the tag to recover the failed run.

The pushed Go tag may already be public through the module proxy even if CI fails or no GitHub Release exists. Never move or rewrite a published tag; source fixes require a new version. For transient network, proxy-propagation, or runner failures, rerun the failed workflow on the same tag.

A manual `workflow_dispatch` with `release_tag` validates tag syntax/module-major compatibility and runs the existing source checks on the selected ref. It does not create a tag, run the tag-only publication job, or create a GitHub Release. Tag creation remains a separate maintainer-authorized action. No publishing credentials or registry tokens are needed for local checks; only the final CI job receives `contents: write`.
