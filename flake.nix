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
        version = pkgs.lib.removeSuffix "\n" (builtins.readFile ./VERSION);
        commit = self.shortRev or self.dirtyShortRev or "unknown";
        sourceDate = self.lastModifiedDate or "19700101000000";
        buildDate =
          "${builtins.substring 0 4 sourceDate}-${builtins.substring 4 2 sourceDate}-${builtins.substring 6 2 sourceDate}T${builtins.substring 8 2 sourceDate}:${builtins.substring 10 2 sourceDate}:${builtins.substring 12 2 sourceDate}Z";
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "githand";
          inherit version;
          src = pkgs.lib.cleanSource self;

          vendorHash = "sha256-T7rmlKIyWUXQOnHf2Tg/w5x378mY+VVq/pu8wb54ohU=";

          env.CGO_ENABLED = 0;

          nativeBuildInputs = [ pkgs.installShellFiles ];

          doCheck = false;

          ldflags = [
            "-s"
            "-w"
            "-X main.version=v${version}"
            "-X main.commit=${commit}"
            "-X main.date=${buildDate}"
          ];

          postInstall = pkgs.lib.optionalString (
            pkgs.stdenv.buildPlatform.canExecute pkgs.stdenv.hostPlatform
          ) ''
            installShellCompletion --cmd githand \
              --bash <($out/bin/githand completion bash) \
              --fish <($out/bin/githand completion fish) \
              --zsh <($out/bin/githand completion zsh)
          '';

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
