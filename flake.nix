{
  description = "note — Typst lecture notes CLI with live preview";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system};
      in {
        packages.default = pkgs.buildGoModule {
          pname = "note";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-M5jZk99qhft+MauB1DFOgdP863/zHIvQ7CtU5DdKnbM=";
          subPackages = [ "cmd/note" ];
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [ go typst tinymist ];
        };
      });
}
