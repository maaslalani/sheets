{
  description = "sheets devshell and package";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = import nixpkgs { inherit system; };
      in {
        devShells.default = pkgs.mkShell {
          name = "sheets-devshell";

          packages = with pkgs; [
            go
            gopls
            gotools
            delve
          ];
        };

        packages.default = pkgs.buildGoModule {
          pname = "sheets";
          version = "0.2.0";

          src = self;

          vendorHash = "sha256-WWtAt0+W/ewLNuNgrqrgho5emntw3rZL9JTTbNo4GsI=";

          subPackages = [ "." ];
          ldflags = [ "-s" "-w" ];

          meta = with pkgs.lib; {
            description = "Terminal based spreadsheet tool";
            license = licenses.mit;
            platforms = platforms.all;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.sheets}/bin/sheets";
        };
      });
}
