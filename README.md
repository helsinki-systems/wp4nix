# wp4nix

So you want to roll out WordPress on NixOS but you also want to use Nix to manage your plugins and themes instead of the builtin plugin system?
You came to the right place.

This default.nix expression contains the code to handle all themes and plugins WordPress has to offer.
It does that by parsing pre-generated JSON files with all plugins and themes.
The files are pre-generated using the `generate.py` script.

## Generating the JSONs

The generate.py script (by default) parses **all** plugins and themes from WordPress.
As the API of WordPress seems to be buggy, not all plugins and themes may be returned, so don't be confused if you get changes if you run the script directly afterwards.
The script takes an optional parameter which may be either `plugin`, `theme` or `language`, to just fetch one of the two categories.
Also, there is a commented-out block in main in case you just want to fetch specific plugins and themes.

There also is an environment varaible, called `COMMIT_LOG`.
If set to `1`, logs are generated.
This is used by the `ci` script.

---

The `ci` script is run daily by our CI and updates all categories.
It basically runs the `generate.py` script and generates a commit message.

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
