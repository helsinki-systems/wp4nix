# wp4nix

So you want to roll out WordPress on NixOS but you also want to use Nix to manage your plugins and themes instead of the builtin plugin system?
You came to the right place.

This default.nix expression contains the code to handle all themes and plugins WordPress has to offer.
It does that by parsing pre-generated JSON files with all plugins and themes.
The files are pre-generated using the code in `main.go` and `svn.go`.

## Generating the JSONs

The go code (by default) parses **all** plugins and themes from WordPress.org.
It does this through a combination of API queries and looking at the subversion repository.

The `WP_VERSION` needs to be set to to the release you want to generate language files for.

You can set the `COMMIT_LOG`, if you want commit logs to be generated.
This is used by the `ci` script.

The amount of workers defaults to 32 unless you are running in debug mode which can be enabled by setting `DEBUG=1`, this can be overridden with the `WORKERS` environment variable.

The parameters `-l`, `-p`, `-t`, `-pl`, `-tl` can be used to specify a comma-separated list of packages to fetch.
The default is to fetch all of them.

## About

We develop this software we made this software for our own usage.
You are free to use it and open issues. We will look through them and decide if this is an issue to our use case, thus we are not able to address all of them.
But do not hesitate to send a pull request!
If you need this software but do not find the time to the development in house, we also offer professional commerical nixOS support - contact us by mail via [kunden@helsinki-systems.de](mailto:kunden@helsinki-systems.de)!

---

The `ci` script is run daily by our CI and updates all categories.
It basically runs the go code, sees if some plugins, themes and languages still build and generates a commit message.

## Using the generated expressions

```nix
{
  nixpkgs.overlays = [ (self: super:
    wordpressPackages = builtins.fetchGit {
      url = "https://git.helsinki.tools/helsinki-systems/wp4nix";
      ref = "master";
    };
  )];
}
```

```sh
$ nix-shell -p wordpressPackages.plugins.woocommerce
```
