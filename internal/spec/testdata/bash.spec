#
# spec file for package bash
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


%define _buildshell /bin/bash
%if 0%{?suse_version} > 1500
%bcond_with alternatives
%else
%bcond_without alternatives
%endif
# Unicode tests do alloc to much memory
%bcond_with altarray
%define rl_major 8
%define rl_version 8.3

%define         bextend %{nil}
%define         bversion 5.3
%define         bpatchlvl %(bash %{_sourcedir}/get_version_number.sh %{_sourcedir})
%global         _incdir     %{_includedir}
%global         _ldldir     %{_libdir}/bash
%global         _minsh      0
%bcond_with     import_function
%bcond_with     sjis
Name:           bash
Version:        %{bversion}.%{bpatchlvl}
Release:        0
# The package bash-completion is a source of
# bugs which will hit at most this package
#Recommends:	bash-completion
License:        GPL-3.0-or-later
URL:            https://www.gnu.org/software/bash/bash.html
# Git:          https://git.savannah.gnu.org/cgit/bash.git
Source0:        https://ftp.gnu.org/gnu/bash/bash-%{bversion}%{bextend}.tar.gz
Source1:        bash-%{bversion}-patches.tar.bz2
Source2:        get_version_number.sh
Source4:        run-tests
Source5:        dot.bashrc
Source6:        dot.profile
Source7:        bash-rpmlintrc
Source8:        baselibs.conf
# Remember unsafe method, compare with
# https://lists.gnu.org/archive/html/bug-bash/2011-03/msg00070.html
# https://lists.gnu.org/archive/html/bug-bash/2011-03/msg00071.html
# https://lists.gnu.org/archive/html/bug-bash/2011-03/msg00073.html
Source9:        bash-4.2-history-myown.dif.bz2
Source10:       https://ftp.gnu.org/gnu/bash/bash-%{bversion}%{bextend}.tar.gz.sig
# GPG key 7C0135FB088AAF6C66C650B9BB5869F064EA74AB Chet Ramey
Source11:       bash.keyring
Source99:       bash.changes
# PATCH-FIX-UPSTREAM bnc#382214 -- disabled due bnc#806628 by -DBNC382214=0
# PATCH-FIX-SUSE For bsc#1065158 add support for broken Japanese locale Shift JIS
# PATCH-FIX-SUSE
# PATCH-EXTEND-SUSE Allow root to clean file system if filled up
# PATCH-FIX-SUSE
# PATCH-UPSTREAM-CHERRY-PICKED
# PATCH-UPSTREAM-MAILINGLIST
# latest bash uses with patch 18 the tag YYEOF
%if %{with alternatives}
%endif
# This has to be always the same version as included in the bash its self
%if %{with alternatives}
%endif
%if %{with alternatives}
%else
%endif

