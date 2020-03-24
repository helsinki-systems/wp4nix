{ pkgs ? import <nixpkgs> {} }:
let
  wp4nix = pkgs.callPackage ./. {};
in
  (with wp4nix.plugins; [
    advanced-excerpt
    elementor
    essential-addons-for-elementor-lite
    geodirectory
    insert-pages
    matomo
    ninja-forms
    woocommerce
    wp-maintenance-mode
    wp-piwik
    wp-smtp
  ])
  ++ (with wp4nix.themes; [
    blogmagazine
    hestia
    kawi
    newsmag
    rocked
    twentyfifteen
    twentytwenty
    vmagazine-lite
  ])
  ++ (with wp4nix.languages; [
    de_DE
    en_GB
    fr_FR
    zh_CN
  ])
