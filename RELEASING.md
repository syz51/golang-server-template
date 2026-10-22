# Releasing the template

Template releases are snapshots consumed by `gonew`; they are not server deployments. Release tags must use canonical Go module versions such as `v1.2.3` or `v1.2.3-rc.1`, without build metadata, with the release's major version matching [Go module-path rules](https://go.dev/ref/mod#major-version-suffixes). The current module path has no `/v2` suffix, so release only v0 or v1 tags until a major-version migration is implemented.

This automated release process targets public module repositories. Private/internal consumers must define their own release process: the public Go module proxy/checksum-database check is not a private-module publication mechanism. The tag publication job still depends on CodeQL succeeding; skipping CodeQL in a private repository does not bypass that dependency.

## Validate a candidate

1. Review the complete diff and ensure the canonical module path and internal imports match the public repository. Run the local checks in [CONTRIBUTING.md](CONTRIBUTING.md); generation checks include relevant uncommitted files.
2. Confirm README commands, supported Go versions, tool pins, `.env.example`, YAML defaults, and license notices match the implementation. Include migration notes for generated projects when configuration or APIs change. Retain upstream MIT notices in source and distributed images.
3. Check private vulnerability reporting in repository settings, or provide a private reporting channel in `SECURITY.md`. Do not assume reporting is enabled; committing these files cannot enable it.
4. Open a pull request for the candidate and wait for **Delivery validation** on its current revision. Linux and macOS native checks, generated-consumer checks, Linux production container checks, vulnerability scanning, and both CodeQL analyses must pass. Confirm that CodeQL uploaded results for Go and GitHub Actions in the repository's code scanning view; a committed workflow is not proof of a successful scan. Review advisory reachability and scan the chosen runtime image according to your publishing environment's policy. Merge through the protected branch workflow, then wait for validation on the exact merged candidate commit before tagging.
5. Set the intended tag and validate its module-major compatibility:

   ```sh
   RELEASE_TAG=v0.1.1 # example only; choose an unused version
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

Pushing the tag triggers all validation jobs again. Only a tag **push** can run the final `publish` job, after `release-tag`, `native`, `generated-consumer`, `container`, `vulnerabilities`, and both `codeql` matrix jobs succeed. The job checks out the tag-push event commit (`github.sha`) and verifies that the remote tag still peels to that commit before running:

```sh
go run ./tools/publiccheck -version "$RELEASE_TAG" -commit "$CANDIDATE_COMMIT"
```

Here `CANDIDATE_COMMIT` is the full event commit SHA. This verifies the exact tag through the public Go module proxy and checksum database, requires the downloaded module origin to match that commit, then checks a generated consumer's race tests, vet, native HTTP behavior, SIGTERM shutdown, and production container. Network access and Docker are mandatory. Public verification is distinct from the working-tree generation check and runs before GitHub Release creation, including on retries. Immediately before any release API operation, CI fetches and compares the remote tag again to detect movement during public tests. These checks bind the event commit, checked-out source, and public module origin; tags must remain immutable because the comparisons cannot atomically exclude every later tag change.

After those checks pass, CI creates a GitHub Release with the tag as its title and generated change notes plus toolchain/platform and consumer-verification notes. Prerelease tags are marked as prereleases. Automation leaves the GitHub **Latest** designation unchanged for all versions, so an older maintenance release cannot displace a newer release; maintainers may choose that designation separately. The workflow uses the current repository identity, so generated projects can reuse it with their own public module/repository and release policy. It publishes no binaries or container images.

Observe **Delivery validation** in the repository's Actions tab and open the release link in the `publish` job summary. Wait for successful public verification and release creation before announcing availability. Recommend the exact verified published tag for repeatable generation; before a tag exists, pin a verified full commit hash. Never describe an unpublished or unverified version as available.

## Repository protection and maintenance

Maintainers must configure and verify repository settings separately; workflow files cannot enable these protections:

- Protect `main` with required pull requests, zero required second-party approvals, required status checks, and restrictions on force pushes and deletion. For the public upstream, require the check names `release-tag`, `native (ubuntu-latest)`, `native (macos-latest)`, `generated-consumer`, `container`, `vulnerabilities`, `CodeQL (go)`, and `CodeQL (actions)`. Private/internal consumers with CodeQL disabled must not require the two CodeQL checks; require them after opting in and verifying successful scans. Confirm these names against the emitted GitHub Actions checks after their first successful run; `publish` is tag-only and must not be a required pull-request check.
- Protect tags matching `v*` against updates and deletion while allowing authorized maintainers to create new version tags.
- Enable immutable releases before publishing future releases. Confirm the new release is marked **Immutable** and its tag matches the validated event commit. Enabling this setting does not make older releases immutable retroactively. See [GitHub's immutable release guidance](https://docs.github.com/en/code-security/concepts/supply-chain-security/immutable-releases).
- Enable Dependabot alerts, Dependabot security updates, and private vulnerability reporting. Use this workflow's advanced CodeQL setup as the source of truth; do not enable a competing default CodeQL setup.

**Delivery validation** runs on all branch pushes, `v*` tag pushes, pull requests (including fork pull requests), manual dispatches, and Mondays at 00:17 UTC. Scheduled runs use the default branch and never publish. CodeQL runs the default queries for Go and GitHub Actions; Go uses a manual `go build ./...` because [CodeQL does not support `none` build mode for Go](https://github.com/github/codeql-action/blob/9b3254085494b55a0f7c4c2b3d2d52cc712ce370/README.md#build-modes). Only the CodeQL job has `security-events: write` and `actions: read`; publication retains its separate `contents: write` permission. Fork analysis uses `pull_request`, with GitHub's fork-token restrictions and no repository secrets or `pull_request_target` execution.

CodeQL runs automatically in public repositories. Private/internal generated repositories skip it by default; enable the licensed [GitHub Code Security capability](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/advanced-security-must-be-enabled) for the repository and confirm policy permits scanning before setting the repository Actions variable `ENABLE_CODEQL` to `true`. The variable alone does not grant a license or upload capability. Verify both uploaded analyses before requiring their checks.

Dependabot checks root Go modules and SHA-pinned GitHub Actions every Monday at 09:00 Asia/Tokyo. Each ecosystem allows up to five open version-update pull requests, groups minor and patch version updates, and leaves major updates separate for review. Security updates depend on the separate repository setting and are not limited to this weekly version-update schedule. Review dependency and action-pin changes through the same required checks; updates are not automatically merged. After merging the configuration, inspect Dependabot's update logs and actual pull requests to verify both ecosystems run successfully.

Use **Create a merge commit** for Dependabot pull requests, not squash or rebase that can inherit the bot's commit authorship. [GitHub documents that Dependabot-authored squash commits on the default branch receive read-only workflow permissions](https://docs.github.com/en/code-security/reference/code-scanning/troubleshoot-analysis-errors/resource-not-accessible). CodeQL skips push events when the actor or head-commit author is Dependabot, while retaining Dependabot/fork pull-request scans and human push/tag scans. Required pull-request checks enforce the public upstream scans before merging. This policy does not enable auto-merge.

## Recovery

Validation failures prevent creation of a GitHub Release. A release API/network error may occur after GitHub accepted creation; inspect the release and rerun the failed workflow without moving tags. Retries revalidate the public consumer, preserve an existing published release without modifying it, and fail visibly if an existing draft needs maintainer attention. Publication is serialized per tag without cancelling an in-progress run.

If either remote-tag comparison fails, publication stops. Investigate the tag movement and publish the intended source under a new unused version; do not retarget the tag to recover the failed run.

The pushed Go tag may already be public through the module proxy even if CI fails or no GitHub Release exists. Never move or rewrite a published tag; source fixes require a new version. For transient network, proxy-propagation, or runner failures, rerun the failed workflow on the same tag.

A manual `workflow_dispatch` with `release_tag` validates tag syntax/module-major compatibility and runs the existing source checks on the selected ref. It does not create a tag, run the tag-only publication job, or create a GitHub Release. Tag creation remains a separate maintainer-authorized action. No publishing credentials or registry tokens are needed for local checks; only the final CI job receives `contents: write`.
