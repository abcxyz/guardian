# Releasing

To build and publish a new version of `guardian`, including publishing binaries for
all supported OSes and architectures, you just push a new tag containing the
version number, as follows.

- Find the previously released version. You can do this by looking at the git
  tags or by looking at the frontpage of this repo on the right side under the
  "Releases" section.
- Figure out the version number to use. We use "semantic versioning"
  (https://semver.org), which means our version numbers look like
  `MAJOR.MINOR.PATCH`. Quoting semver.org:

        increment the MAJOR version when you make incompatible API changes
        increment the MINOR version when you add functionality in a backward compatible manner
        increment the PATCH version when you make backward compatible bug fixes

  The most important thing is that if we change an API or command-line user
  journey in a way that could break an existing use-case, we must increment the
  major version.

- Push a signed tag in git, with the tag named with your version number, with a
  message saying why you're creating this release. For example:

      $ git tag -s -a v0.2.1 -m 'Fixed a bug that caused plan to not create PR comments'
      $ git push origin v0.2.1

- A GitHub workflow will be triggered by the tag push and will handle
  everything. You will see the new release created within a few minutes.
