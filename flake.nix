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
        runtimeLibs = pkgs.lib.makeLibraryPath [ pkgs.vulkan-loader pkgs.libGL ];
      in
      {
        packages.default = pkgs.buildGoModule {
          pname = "gdraw";
          version = "0.1.0";
          src = ./.;
          vendorHash = "sha256-vt+VebX1tQW91zyfCYRVoviDFQr1MmKuDvBGcSk+G/c=";
          nativeBuildInputs = [ pkgs.pkg-config pkgs.makeWrapper ];
          buildInputs = libs;
          subPackages = [ "." ];
          doCheck = false;
          postFixup = ''
            wrapProgram $out/bin/gdraw --prefix LD_LIBRARY_PATH : ${runtimeLibs}
          '';
          meta = {
            description = "A minimalist canvas for quick drawings";
            homepage = "https://github.com/jim-ww/gdraw";
            license = pkgs.lib.licenses.gpl3Only;
            mainProgram = "gdraw";
          };
        };

        devShells.default = pkgs.mkShell {
          packages = [ pkgs.go pkgs.pkg-config ] ++ libs;
          LD_LIBRARY_PATH = runtimeLibs;
        };
      });
}
