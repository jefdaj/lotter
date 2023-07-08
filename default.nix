with import <nixpkgs> {};

pkgs.buildGoModule {
  pname = "lotter";
  version = "395b62bc7d";
  src = ./.;
  vendorHash = null;
}
