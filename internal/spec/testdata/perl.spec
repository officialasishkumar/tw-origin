#
# spec file
#
# Copyright (c) 2025 SUSE LLC
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


%define flavor @BUILD_FLAVOR@%{nil}
%if "%{?flavor}" != "" && "%{?flavor}" != ("@BUILD_" + "FLAVOR@")
%define name_suffix -%{flavor}
%else
%define name_suffix %{nil}
%endif

%define versionlist_provides() %{lua:for i, n in ipairs(arg) do ; print(" perl(:MODULE_COMPAT_"..n..")") ; end}

%define pversion 5.44.0
# set to nil when equal to pversion
%global versionlist %nil
Name:           perl%{?name_suffix}
Version:        5.44.0
Release:        0
License:        Artistic-1.0 OR GPL-1.0-or-later
URL:            https://www.perl.org/
Source:         https://www.cpan.org/src/5.0/perl-%{version}.tar.xz
Source1:        perl-rpmlintrc
Source2:        macros.perl
Source3:        README.macros
Source4:        baselibs.conf
Source100:      README.md
# PATCH-FIX-OPENSUSE perl-5.22.0_wrong_rpm_provides.diff Prevent generating bad RPM provides
# PATCH-FIX-OPENSUSE prevent changing package when rebuilding
# PATCH-FIX-OPENSUSE skip flaky tests powerpc as bypass https://bugzilla.suse.com/show_bug.cgi?id=1063176
# PATCH-FIX-UPSTREAM unmerged https://www.nntp.perl.org/group/perl.perl5.porters/2018/12/msg253240.html
# For t/op/magic and dist/threads/t/join - but because perl is a bootstrap package don't use it
#BuildRequires:  procps
%if "%{versionlist}" != ""
%endif
%if "%{version}" != "%{pversion}"
%endif

