{ pkgs, ... }:

{
  packages = [
    pkgs.go_1_27
    pkgs.just
    pkgs.golangci-lint
    pkgs.oxfmt
  ];

  env.GOTOOLCHAIN = "local";
}
