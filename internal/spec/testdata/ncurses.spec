#
# spec file for package ncurses
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


%bcond_with     hasheddb
%if 0%{?suse_version} > 1320
%bcond_without  symversion
%else
%bcond_with     symversion
%endif
%bcond_with     memleakck
%bcond_without  onlytinfo
%bcond_without  abi5
%bcond_with     abi7
%bcond_with     ada
%bcond_with     libbsd
%bcond_with     usepcre2
%bcond_with     gpgverify

%if %{with onlytinfo}
%global soname_tinfo tinfo
%else
%global soname_tinfo tinfow
%endif
%ifarch s390x s390
%global fallback unknown,dumb,xterm,xterm-256color,ibm327x,ms-terminal,vt100,vt102,vt220,sclp
%else
%global fallback unknown,dumb,xterm,xterm-256color,linux,ms-terminal,vt100,vt102,vt220
%endif

%global patchlvl %(bash %{_sourcedir}/get_version_number.sh %{_sourcedir})
%global basevers 6.6
%global tackvers 1.11
%global tacklvl  20251210
%global download invisible-island.net
%global glibc %(rpm -q --queryformat '%%{VERSION}' -f /lib64/libc.so 2>/dev/null | sed 's/\.//g')
# ${!...} and pushd is bash-specific
%define _buildshell /bin/bash

Name:           ncurses
#!BuildIgnore: terminfo
%if %{with hasheddb}
%endif
%if %{with gpgverify}
%endif
%if %{with ada}
# Currently we're missing gprbuild and gprconfig
%endif
%if %{with libbsd}
%endif
%if %{with usepcre2}
%endif
%define terminfo() %{_datadir}/%{0}/%{1}
%define tabset()   %{_datadir}/%{0}/%{1}
# bug437293
%ifarch ppc64
%endif
#
# Note that this spec files does not only build the ABI version 6
# but also build the ABI version 5 as this is part of the source
# tar ball including the latest upstream fixes for ABI 5.
#
Version:        %{basevers}.%{patchlvl}
Release:        0
#Git:           http://ncurses.scripts.mit.edu
License:        MIT
URL:            https://www.invisible-island.net/ncurses/ncurses.html
Source0:        https://%{download}/archives/ncurses/ncurses-%{basevers}.tar.gz
Source1:        ncurses-%{basevers}-patches.tar.bz2
Source2:        handle.linux
Source3:        README.devel
Source4:        ncurses-rpmlintrc
# Latest tack can be found at ftp://ftp.invisible-island.net/pub/ncurses/current/
Source5:        https://%{download}/archives/ncurses/current/tack-%{tackvers}-%{tacklvl}.tgz
Source6:        edit.sed
Source7:        baselibs.conf
Source8:        cursescheck
Source9:        https://%{download}/archives/ncurses/ncurses-%{basevers}.tar.gz.asc
Source10:       https://%{download}/archives/ncurses/current/tack-%{tackvers}-%{tacklvl}.tgz.asc
Source11:       ncurses.keyring
Source12:       ncursesnt
Source42:       termerase.c
Source43:       termerase.1
BuildRoot:      %{_tmppath}/%{name}-%{version}-build
%global         _miscdir    %{_datadir}/misc
%global         _incdir     %{_includedir}
%global         root        %{_tmppath}/%{name}-%{version}-store

