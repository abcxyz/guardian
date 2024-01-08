## Release guide for team members

To build and publish a new version of `guardian`, including publishing binaries
for all supported OSes and architectures, you push a new tag containing the
version number, as follows:

- Find the previously released version. You can do this by looking at the git
  tags or by looking at the
  [`"Releases"`](https://github.com/abcxyz/guardian/releases) page for this
  repo.

- Figure out the version number to use. We use "semantic versioning"
  (https://semver.org), which means our version numbers look like
  `MAJOR.MINOR.PATCH`. Quoting semver.org:

        increment the MAJOR version when you make incompatible API changes
        increment the MINOR version when you add functionality in a backward compatible manner
        increment the PATCH version when you make backward compatible bug fixes

  The most important thing is that if we change an API or command-line user
  journey in a way that could break an existing use-case, we must increment the
  major version.

> [!IMPORTANT]
> **Before** creating the release, ensure all templates in the
> [`abc.templates`](./abc.templates) folder are updated to the new version.

- Run the
  [create tag workflow](https://github.com/abcxyz/guardian/actions/workflows/create-tag.yml)
  using the version number you've decided on. It's OK to leave the "message"
  field blank. Another team member will have to approval the workflow run

- A GitHub workflow will be triggered by the tag push and will handle
  everything. You will see the new release created within a few minutes. If not,
  look for failed
  [Release workflow runs](https://github.com/abcxyz/guardian/actions/workflows/release.yml)
  and look at their logs.
