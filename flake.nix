{
  description = "zmx session manager";

  inputs = {
    nixpkgs.url = "github:nixos/nixpkgs/nixos-unstable";
    utils.url = "github:numtide/flake-utils";
    flake-compat = {
      url = "github:NixOS/flake-compat";
      flake = false;
    };
  };

  outputs = { self, nixpkgs, utils, ... }:
    let
      supportedSystems = [
        "x86_64-linux"
        "aarch64-linux"
        "aarch64-darwin"
      ];
      overlay = final: _prev:
        let
          buildDate = self.lastModifiedDate or "unknown";
          revision = self.shortRev or "none";
        in
        {
          zsm = final.buildGoModule rec {
            pname = "zsm";
            version = "unstable-${buildDate}";
            src = ./.;

            vendorHash = "sha256-CE8atBJ7TJ6RI2uJDvPOwT4kRmyFEGC/vwOm7EoJL6U=";

            ldflags = [
              "-s"
              "-w"
              "-X main.version=${version}"
              "-X main.commit=${revision}"
              "-X main.date=${buildDate}"
            ];

            subPackages = [ "." ];

            postInstall = ''
              mv $out/bin/zmx-session-manager $out/bin/zsm
            '';

            meta = with final.lib; {
              description = "zmx session manager";
              homepage = "https://github.com/mdsakalu/zmx-session-manager";
              license = licenses.mit;
              mainProgram = "zsm";
            };
          };
        };
    in
    {
      overlays.default = overlay;
    } // utils.lib.eachSystem supportedSystems (system:
      let
        pkgs = import nixpkgs {
          inherit system;
          overlays = [ overlay ];
        };
      in
      {
        packages.default = pkgs.zsm;

        devShells.default = pkgs.mkShell {
          buildInputs = with pkgs; [
            go
            gofumpt
            golangci-lint
          ];
        };
      }
    );
}
