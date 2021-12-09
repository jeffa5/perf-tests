{
  description = "Kubernetes performance tests";

  inputs = {
    flake-utils = {
      url = "github:numtide/flake-utils";
    };
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let pkgs = nixpkgs.legacyPackages.${system}; in
      {
        packages.clusterloader2 = pkgs.buildGoModule {
          pname = "clusterloader2";
          version = "0.1.0";
          src = ./clusterloader2;
          vendorSha256 = "sha256-dk9LdI+VnJlP13FWv1F0xubT9YKu5m9wQEYKTD+nuIw=";
          postInstall = ''
            mv $out/bin/cmd $out/bin/clusterloader2
          '';
          runTests = false;
        };

        defaultPackage = self.packages.${system}.clusterloader2;

        apps.clusterloader2 = flake-utils.lib.mkApp {
          drv = self.packages.${system}.clusterloader2;
        };

        defaultApp = self.apps.${system}.clusterloader2;

        devShell = pkgs.mkShell {
          packages = with pkgs;
            [
              go_1_15
              kind
              k9s
              kubectl
              helm

              self.packages.${system}.clusterloader2

              nixpkgs-fmt
              rnix-lsp
            ];
        };
      });
}
