## Release Process

[Semantic Versioning 2.0.0 reference](https://github.com/semver/semver/blob/master/semver.md)

### 1. New Feature or Breaking‑Change Release (Minor/Major)

1. **Merge & Verify**
- Merge all feature or breaking‑change PRs into `master`.
- Ensure CI (tests, linter, codegen) all pass on `master`.

2. **Determine Version Bump**
- **Major** (`X.0.0`) when you make incompatible changes
- **Minor** (`0.Y.0`) when you add functionality in a backward compatible manner
- **Patch** (`0.0.Z`) when you make backward compatible bug fixes

3. **Create Git Tag**
   ```bash
   git tag vX.Y.Z
   git push origin vX.Y.Z
   ```
   This triggers the `build-and-release` workflow.

5. **Monitor Draft Release**
   - GitHub Actions will:
     - Run `go generate ./…`
     - Build artifacts with `main.version=vX.Y.Z`
     - Zip as `go-mud-release-vX.Y.Z.zip`
     - Draft a GitHub Release named `vX.Y.Z`

6. **Finalize Release Notes**
   - Review and adjust the draft on GitHub, then click **Publish release**.

7. **Announce**
   - Share the release link with the team or via configured notifications.

---

### 2. Basic Patch Release (x.y.Z)

1. **Merge Bug‑Fix PR**
   - Once the fix is in `master` and CI is green.

2. **Determine Patch Bump**
   ```bash
   # if current version is vX.Y.Z:
   git tag vX.Y.(Z+1)
   git push origin vX.Y.(Z+1)
   ```

3. **Tag & Push**
   - Pushing the tag triggers the same workflow.

4. **Publish**
   - Review draft release, then click **Publish release**.

---

### FAQ / Guidelines

- **Does every merge to `master` trigger a release?**
  No – only pushing a Git tag matching `v*.*.*` triggers a release.

- **When should I bump minor vs. patch?**
  - **Minor** for new, backward‑compatible features.
  - **Patch** for bug fixes or documentation tweaks.

- **What about `go generate` directives?**
  The workflow runs `go generate ./…` automatically before each build.
