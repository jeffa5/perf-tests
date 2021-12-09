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
        devShell = pkgs.mkShell {
          packages = with pkgs; [
            go_1_15
            kind
            k9s

            nixpkgs-fmt
            rnix-lsp
          ];
        };
      });
}
