# ==================================================
#  freebuff-cli -- standalone Go launcher for Freebuff CLI
#
#  Replaces the legacy Node.js / npx wrapper with a zero-dependency compiled Go
#  binary (`freebuff-cli`) that queries `registry.npmjs.org/freebuff` directly,
#  verifies SHA-256 checksums, manages `~/.config/manicode/freebuff`, and
#  provides a Kitty desktop entry plus optional systemd update-check timer.
#
#  Note: The downloaded upstream `~/.config/manicode/freebuff` binary is a
#  generic dynamically linked ELF, so `programs.nix-ld.enable = true;` is still
#  required in your NixOS system configuration.
# ==================================================
{
  config,
  lib,
  pkgs,
  ...
}: let
  cfg = config.programs.freebuff-cli;

  freebuffCliPkg = pkgs.buildGoModule {
    pname = "freebuff-cli";
    version = "1.0.0";
    src = ./..;
    vendorHash = null;
    env.CGO_ENABLED = 0;
    subPackages = ["cmd/freebuff-cli"];
    ldflags = ["-s" "-w"];
  };
in {
  options.programs.freebuff-cli = {
    enable = lib.mkEnableOption "the standalone Go freebuff-cli coding agent launcher";

    desktopEntry.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Install a desktop icon entry that starts freebuff-cli inside the Kitty terminal.";
    };

    icon = lib.mkOption {
      type = lib.types.str;
      default = "utilities-terminal";
      description = "Icon name or path for the Freebuff CLI desktop entry.";
    };

    notifications.enable = lib.mkOption {
      type = lib.types.bool;
      default = true;
      description = "Send a desktop notification via notify-send when a newer npm release is published.";
    };

    checkForUpdates = {
      enable = lib.mkOption {
        type = lib.types.bool;
        default = true;
        description = "Run periodic background update checks from a systemd user timer.";
      };

      interval = lib.mkOption {
        type = lib.types.str;
        default = "4h";
        description = "How often the systemd user timer checks for a newer upstream release.";
      };
    };
  };

  config = lib.mkIf cfg.enable {
    home.packages = [
      freebuffCliPkg
      pkgs.kitty
      pkgs.libnotify
    ];

    xdg.desktopEntries.freebuff-cli = lib.mkIf cfg.desktopEntry.enable {
      name = "Freebuff CLI";
      genericName = "AI Coding Agent";
      comment = "Freebuff AI Coding Agent (CLI in Kitty)";
      icon = cfg.icon;
      exec = "${pkgs.kitty}/bin/kitty --title \"Freebuff CLI\" --class freebuff-cli -e ${freebuffCliPkg}/bin/freebuff-cli";
      terminal = false;
      categories = ["Development" "Utility" "ConsoleOnly"];
      settings = {
        TryExec = "${pkgs.kitty}/bin/kitty";
        StartupWMClass = "freebuff-cli";
        Keywords = "ai;coding;agent;freebuff;cli;terminal;";
      };
    };

    systemd.user.services.freebuff-cli-check = lib.mkIf cfg.checkForUpdates.enable {
      Unit.Description = "Check freebuff-cli for a newer upstream npm release";
      Service = {
        Type = "oneshot";
        Environment = [
          "FREEBUFF_CLI_NOTIFY_SEND=${pkgs.libnotify}/bin/notify-send"
        ];
        ExecStart =
          "${freebuffCliPkg}/bin/freebuff-cli "
          + (
            if cfg.notifications.enable
            then "--notify"
            else "--check"
          );
        StandardOutput = "null";
        StandardError = "null";
      };
    };

    systemd.user.timers.freebuff-cli-check = lib.mkIf cfg.checkForUpdates.enable {
      Unit.Description = "Check freebuff-cli for a newer upstream release every ${cfg.checkForUpdates.interval}";
      Timer = {
        OnStartupSec = "2min";
        OnUnitActiveSec = cfg.checkForUpdates.interval;
        Unit = "freebuff-cli-check.service";
      };
      Install.WantedBy = ["timers.target"];
    };
  };
}
