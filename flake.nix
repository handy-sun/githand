{
  description = "Git workspace sync & migration CLI — scan, status, snapshot, restore";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixpkgs-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        version = builtins.substring 0 7 self.rev or "dirty";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "githand";
          inherit version;
          src = pkgs.lib.cleanSource self;

          vendorHash = "sha256-T7rmlKIyWUXQOnHf2Tg/w5x378mY+VVq/pu8wb54ohU=";

          env.CGO_ENABLED = 0;

          doCheck = false;

          ldflags = [
            "-s"
            "-w"
            "-X main.version=${version}"
            "-X main.commit=${version}"
          ];

          meta = with pkgs.lib; {
            description = "Git workspace sync & migration CLI";
            homepage = "https://github.com/handy-sun/githand";
            license = licenses.mit;
            mainProgram = "githand";
            platforms = platforms.unix ++ platforms.windows;
          };
        };

        devShells.default = pkgs.mkShell {
          packages = with pkgs; [
            go
            gopls
            goreleaser
          ];

          shellHook = ''
            echo "githand dev shell — go $(go version | awk '{print $3}')"
          '';
        };
      });
}