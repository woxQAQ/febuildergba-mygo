{ pkgs, lib, ... }:

{
  packages = with pkgs; [
    go_1_27
    gopls
    just
    golangci-lint
    oxfmt
  ];

  env.GOTOOLCHAIN = "local";

  # 未打包的 Go 程序没有 wrapGAppsHook 包装，需要提供 GTK 的设置定义。
  enterShell = lib.optionalString pkgs.stdenv.hostPlatform.isLinux ''
    export XDG_DATA_DIRS="${pkgs.gtk3}/share/gsettings-schemas/${pkgs.gtk3.name}:${pkgs.gsettings-desktop-schemas}/share/gsettings-schemas/${pkgs.gsettings-desktop-schemas.name}:''${XDG_DATA_DIRS:-/usr/local/share:/usr/share}"
  '';

  # MyGo 使用 purego 按库名加载 Linux 原生依赖，需要显式提供搜索路径。
  env.LD_LIBRARY_PATH = lib.mkIf pkgs.stdenv.hostPlatform.isLinux (
    lib.makeLibraryPath (
      with pkgs;
      [
        glib
        gtk3
        cairo
        gdk-pixbuf
        pango
        harfbuzz
        fontconfig
        at-spi2-core
        libepoxy
        libGL
        libx11
      ]
    )
    # NixOS 的图形驱动由系统提供，与当前内核及硬件保持一致。
    + ":/run/opengl-driver/lib"
  );
}
