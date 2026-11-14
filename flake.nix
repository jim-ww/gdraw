{
  description = "gdraw - a minimalist canvas for quick drawings";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
    flake-utils.url = "github:numtide/flake-utils";
  };

  outputs = { self, nixpkgs, flake-utils }:
    flake-utils.lib.eachDefaultSystem (system:
      let
        pkgs = nixpkgs.legacyPackages.${system};
        libs = with pkgs; [
          vulkan-headers
          libxkbcommon
          wayland
          libx11
          libxcb
          libxcursor
          libxfixes
          libGL
        ];
      in
      {
        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.pkg-config ] ++ libs;
          LD_LIBRARY_PATH = "${pkgs.vulkan-loader}/lib:${pkgs.libGL}/lib";
        };
      });
}
