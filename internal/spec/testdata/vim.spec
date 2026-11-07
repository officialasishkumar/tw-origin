#
# spec file for package vim
#
# Copyright (c) 2026 SUSE LLC and contributors
#
# All modifications and additions to the file contributed by third parties
# remain the property of their copyright owners, unless otherwise agreed
# upon. The license for this file, and modifications and additions to the
# file, is the same license as for the pristine package itself (unless the
# license for the pristine package is not an Open Source License, in which
# case the license is the MIT License). An "Open Source License" is a
# license that conforms to the Open Source Definition (Version 1.9)
# published by the Open Source Initiative.

# Please submit bugfixes or comments via https://bugs.opensuse.org/
#


%define pkg_version 9.2
%define patchlevel 1202
%define patchlevel_compact %{patchlevel}
%define VIM_SUBDIR vim92
%define site_runtimepath %{_datadir}/vim/site
%define make make VIMRCLOC=%{_sysconfdir} VIMRUNTIMEDIR=%{_datadir}/vim/current MAKE="make -e" %{?_smp_mflags}
%if 0%{?suse_version} > 1500
%bcond_without libalternatives
%else
%bcond_with libalternatives
%endif
Name:           vim
Version:        %{pkg_version}.%{patchlevel_compact}
Release:        0
License:        Vim
URL:            https://www.vim.org/
Source:         https://github.com/vim/vim/archive/v%{pkg_version}.%{patchlevel}.tar.gz#/vim-%{pkg_version}.%{patchlevel}.tar.gz
Source3:        suse.vimrc
Source4:        vimrc_example1
Source5:        vimrc_example2
Source8:        suse.gvimrc
Source10:       README.Japanese-XIM
Source13:       vitmp.c
Source14:       vitmp.1
Source15:       vim132
Source19:       gvim.desktop
Source20:       spec.skeleton
Source21:       spec.vim
Source23:       apparmor.vim
# Generated using: epstopdf runtime/vimlogo.eps && pdf2svg runtime/vimlogo.pdf gvim.svg
Source24:       gvim.svg
# Section Bitmap icons generated from gvim.svg using rsvg-convert
Source25:       gvim_24.png
Source26:       gvim_32.png
Source27:       gvim_48.png
Source28:       gvim_64.png
Source29:       gvim_96.png
Source30:       gvim_128.png
Source31:       gvim_256.png
Source32:       gvim_512.png
# /Section
Source99:       %{name}-7.4-rpmlintrc
#!BuildIgnore:  glycin-loaders
%{?libperl_requires}
%if %{with libalternatives}
%else
%endif

