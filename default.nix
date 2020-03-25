{ fetchsvn, unzip, lib, stdenvNoCC, wp-cli, gettext }: with lib;
let packages = (self:
  let
    json = {
      plugins = builtins.fromJSON (readFile ./plugins.json);
      themes = builtins.fromJSON (readFile ./themes.json);
      languages = builtins.fromJSON (readFile ./languages.json);
    };
    filterFileName = f: builtins.replaceStrings [ " " "," "/" "&" ";" ''"'' "'" "$" ":" "(" ")" "[" "]" "{" "}" "|" "*" "\t" ] [ "_" "." "." "" "" "" "" "" "" "" "" "" "" "" "" "-" "" "" ] f;
    fetch = t: v: fetchsvn {
      inherit (v) rev sha256;
      url = if t == "plugins" || t == "themes" then
        "https://" + t + ".svn.wordpress.org/" + v.path
      else if t == "languages" then
        "https://i18n.svn.wordpress.org/core/" + v.version + "/" + v.path
      else
        throw "invalid fetch type";
    };
    mkPkg = type: pname: value: stdenvNoCC.mkDerivation ({
      inherit pname;
      version = filterFileName value.version;
      src = fetch type value;
      installPhase = ''
        cp -R ./. $out
      '';
    } // optionalAttrs (type == "languages") {
      nativeBuildInputs = [ gettext wp-cli ];
      buildPhase = ''
        find -name '*.po' -print0 | while IFS= read -d "" -r po; do
          msgfmt -o $(basename "$po" .po).mo "$po"
        done
        wp i18n make-json .
        rm *.po
      '';
    });
  in
    genAttrs [ "plugins" "themes" "languages" ] (t: mapAttrs (mkPkg t) json."${t}")
);
in makeExtensible packages
