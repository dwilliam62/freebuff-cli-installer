{
  description = "Standalone cross-distro Go launcher and updater for Freebuff CLI (zero Node/npm dependencies)";

  inputs = {
    nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";
  };

  outputs = {
    self,
    nixpkgs,
  }: let
    supportedSystems = ["x86_64-linux" "aarch64-linux"];
    forAllSystems = nixpkgs.lib.genAttrs supportedSystems;
  in {
    packages = forAllSystems (system: let
      pkgs = nixpkgs.legacyPackages.${system};
    in {
      default = self.packages.${system}.freebuff-cli;
      freebuff-cli = pkgs.buildGoModule {
        pname = "freebuff-cli";
        version = "1.0.0";
        src = ./.;
        vendorHash = null;
        env.CGO_ENABLED = 0;
        subPackages = ["cmd/freebuff-cli"];
        ldflags = ["-s" "-w"];

        postInstall = ''
          mkdir -p $out/share/applications
          cat > $out/share/applications/freebuff-cli.desktop <<EOF
          [Desktop Entry]
          Type=Application
          Name=Freebuff CLI
          GenericName=AI Coding Agent
          Comment=Freebuff AI Coding Agent (CLI in Kitty)
          Icon=utilities-terminal
          TryExec=kitty
          Exec=kitty --title "Freebuff CLI" --class freebuff-cli -e $out/bin/freebuff-cli
          Terminal=false
          Categories=Development;Utility;ConsoleOnly;
          Keywords=ai;coding;agent;freebuff;cli;terminal;
          StartupWMClass=freebuff-cli
          EOF
        '';

        meta = with pkgs.lib; {
          description = "Standalone Go launcher and updater for the Freebuff AI coding agent";
          homepage = "https://freebuff.com";
          license = licenses.gpl3Plus;
          mainProgram = "freebuff-cli";
          platforms = platforms.linux;
        };
      };
    });

    homeManagerModules.default = import ./nixos/freebuff-cli.nix;
    homeManagerModules.freebuff-cli = import ./nixos/freebuff-cli.nix;
  };
}
