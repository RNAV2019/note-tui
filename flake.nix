{
  description = "note — Typst lecture notes TUI with live preview";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = {
    self,
    nixpkgs,
    flake-utils,
  }:
    flake-utils.lib.eachDefaultSystem (system: let
      pkgs = nixpkgs.legacyPackages.${system};
      version = "0.2.0";
    in {
      packages.default = pkgs.buildGoModule {
        pname = "note";
        inherit version;
        src = ./.;
        vendorHash = "sha256-qZ43J6cJQRMEQFHj4fZA5wsGYDC7vWIHfly1VZ/gVJ8=";
        subPackages = ["cmd/note"];
        ldflags = ["-s" "-w" "-X main.version=${version}"];
      };

      devShells.default = pkgs.mkShell {
        packages = with pkgs; [go typst tinymist];
      };
    });
}
