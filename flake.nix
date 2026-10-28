{
  description = "Hikari game launcher for NixOS";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
    in
    {
      packages = forAllSystems (system:
        let
          pkgs = import nixpkgs { inherit system; };
          inherit (pkgs) lib;

          # Hikari requires Go 1.27.3 or newer.
          goArch = if system == "x86_64-linux" then "amd64" else "arm64";

          # Reproducible fallback when pinned nixpkgs lacks Go 1.27.3.
          fallbackGo = pkgs.runCommand "go-1.27.3" {
            nativeBuildInputs = [ pkgs.gnutar pkgs.gzip ];
            passthru = {
              GOOS = "linux";
              GOARCH = goArch;
              CGO_ENABLED = "1";
            };
          } ''
            mkdir -p "$out"
            tar -xzf ${pkgs.fetchurl {
              url = "https://go.dev/dl/go1.27.3.linux-${goArch}.tar.gz";
              hash =
                if system == "x86_64-linux"
                then "sha256-Ze+8gP3B+izSYxytW0gXd6aOhyXzTon1D8ascU43m0A="
                else "sha256-oG6ZMJ28vcoajM0igsPuFG5VkwiG4mtPEZMmlYcWX0k=";
            }} --strip-components=1 -C "$out"
          '';

          goToolchain =
            if pkgs ? go_latest
               && lib.versionAtLeast pkgs.go_latest.version "1.27.3"
            then pkgs.go_latest
            else fallbackGo;

          buildHikariGoModule =
            pkgs.buildGoModule.override { go = goToolchain; };
        in
        rec {
          hikari = buildHikariGoModule {
            pname = "hikari";
            version = "unstable";
            src = ./.;

            # Hikari commits its Go dependencies to vendor/.
            vendorHash = null;
            subPackages = [ "cmd/hikari" "cmd/hikarid" ];

            nativeBuildInputs = [ pkgs.makeWrapper ];
            nativeCheckInputs = [ pkgs.python3 ];

            postInstall = ''
              install -Dm755 scripts/hikari-app "$out/bin/hikari-app"
              install -Dm644 packaging/hikari.desktop \
                "$out/share/applications/hikari.desktop"
              install -Dm644 packaging/hikari.svg \
                "$out/share/icons/hicolor/scalable/apps/hikari.svg"

              install -d "$out/etc/xdg/quickshell/hikari"
              cp -r quickshell/. "$out/etc/xdg/quickshell/hikari/"

              install -Dm644 packaging/hikarid.service \
                "$out/lib/systemd/user/hikarid.service"
              substituteInPlace "$out/lib/systemd/user/hikarid.service" \
                --replace-fail 'ExecStart=/usr/bin/hikarid' \
                "ExecStart=$out/bin/hikarid"

              wrapProgram "$out/bin/hikarid" \
                --prefix PATH : ${lib.makeBinPath [ pkgs.python3 ]}

              wrapProgram "$out/bin/hikari-app" \
                --prefix PATH : ${lib.makeBinPath [ pkgs.quickshell pkgs.coreutils pkgs.systemd ]} \
                --prefix XDG_CONFIG_DIRS : "$out/etc/xdg"
            '';

            meta = with lib; {
              description = "Native game launcher with a Go daemon and Quickshell frontend";
              homepage = "https://github.com/dodo986/hikari";
              license = licenses.gpl3Plus;
              mainProgram = "hikari-app";
              platforms = platforms.linux;
            };
          };

          default = hikari;
        });

      apps = forAllSystems (system: {
        default = {
          type = "app";
          program = "${self.packages.${system}.hikari}/bin/hikari-app";
        };
      });
    };
}
