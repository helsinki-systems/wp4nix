with import <nixpkgs> { };

stdenv.mkDerivation {
  name = "go";
  buildInputs = [
    delve
    libcap
    go
    gcc
    subversion
  ];
  shellHook = ''
    export GOPATH=$PWD/gopath
  '';
}
