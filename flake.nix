{
  description = "Flake for github:maaslalani/sheets";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-25.11";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs =
    {
      self,
      nixpkgs,
      flake-utils,
    }:
    flake-utils.lib.eachDefaultSystem (
      system:
      let
        pkgs = import nixpkgs { inherit system; };
      in
      {
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

          vendorHash = "sha256-X7bfALH9mM15HP6SM60CFIG1rm4Ma6LEh2p7z5LNW64=";

          subPackages = [ "." ];
          ldflags = [
            "-s"
            "-w"
          ];

          meta = with pkgs.lib; {
            description = "Terminal based spreadsheet tool";
            license = licenses.mit;
            platforms = platforms.all;
          };
        };

        apps.default = {
          type = "app";
          program = "${self.packages.${system}.default}/bin/sheets";
        };
      }
    );
}
