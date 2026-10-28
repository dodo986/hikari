{
  description = "Hikari game launcher for NixOS";

  inputs.nixpkgs.url = "github:NixOS/nixpkgs/nixos-unstable";

  outputs = { self, nixpkgs }:
    let
      systems = [ "x86_64-linux" "aarch64-linux" ];
      forAllSystems = nixpkgs.lib.genAttrs systems;
      version = "0.8.4";
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

          # The React bundle the window embeds. Dependencies come from the
          # lockfile, so this derivation keeps no hand-written hash.
          web = pkgs.stdenv.mkDerivation {
            pname = "hikari-web";
            inherit version;
            src = ./frontend/web;

            nativeBuildInputs = [ pkgs.nodejs_22 pkgs.importNpmLock.npmConfigHook ];
            npmDeps = pkgs.importNpmLock { npmRoot = ./frontend/web; };

            buildPhase = ''
              runHook preBuild
              npm run build
              runHook postBuild
            '';

            installPhase = ''
              runHook preInstall
              mkdir -p "$out"
              cp -r dist/. "$out"
              runHook postInstall
            '';
          };

          hikari-ui = buildHikariGoModule {
            pname = "hikari-ui";
            inherit version;
            src = ./frontend/host;

            vendorHash = "sha256-f5dN6aIZ0uMGanKqKd4SUeT/jHbwCYjRRllxRlHhtNc=";
            subPackages = [ "." ];
            tags = [ "production" "webkit2_41" ];

            buildInputs = [ pkgs.gtk3 pkgs.webkitgtk_4_1 ];
            nativeBuildInputs = [ pkgs.pkg-config pkgs.wrapGAppsHook3 ];

            preBuild = ''
              rm -rf dist
              cp -r ${web} dist
            '';

            # The module directory is named host, so name the binary after the
            # window the packaging expects.
            postInstall = ''
              mv "$out/bin/host" "$out/bin/hikari-ui"
            '';

            meta = with lib; {
              description = "Hikari's React window inside a WebKitGTK host";
              homepage = "https://github.com/dodo986/hikari";
              license = licenses.gpl3Plus;
              mainProgram = "hikari-ui";
              platforms = platforms.linux;
            };
          };
        in
        rec {
          inherit hikari-ui;

          hikari = buildHikariGoModule {
            pname = "hikari";
            inherit version;
            src = ./.;

            # Hikari commits its Go dependencies to vendor/.
            vendorHash = null;
            subPackages = [ "cmd/hikari" "cmd/hikarid" ];

            nativeBuildInputs = [ pkgs.makeWrapper ];
            nativeCheckInputs = [ pkgs.python3 ];

            postInstall = ''
              install -Dm755 scripts/hikari-app "$out/bin/hikari-app"
              install -Dm755 scripts/hikari-update "$out/bin/hikari-update"
              install -Dm755 ${hikari-ui}/bin/hikari-ui "$out/bin/hikari-ui"

              install -Dm644 packaging/hikari.desktop \
                "$out/share/applications/hikari.desktop"
              substituteInPlace "$out/share/applications/hikari.desktop" \
                --replace-fail 'hikari-ui' "$out/bin/hikari-app"
              install -Dm644 packaging/hikari.svg \
                "$out/share/icons/hicolor/scalable/apps/hikari.svg"

              install -Dm644 packaging/hikarid.service \
                "$out/lib/systemd/user/hikarid.service"
              substituteInPlace "$out/lib/systemd/user/hikarid.service" \
                --replace-fail 'ExecStart=/usr/bin/hikarid' \
                "ExecStart=$out/bin/hikarid"

              wrapProgram "$out/bin/hikarid" \
                --prefix PATH : ${lib.makeBinPath [ pkgs.python3 ]}

              wrapProgram "$out/bin/hikari-app" \
                --prefix PATH : ${lib.makeBinPath [ pkgs.coreutils pkgs.systemd ]}
            '';

            meta = with lib; {
              description = "Native game launcher with a Go daemon and a React window";
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
