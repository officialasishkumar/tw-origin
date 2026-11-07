#
# spec file for package zypper.spec
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


Name:           zypper
Version:        1.14.102
Release:        0
Source:         %{name}-%{version}.tar.bz2
Source1:        %{name}-rpmlintrc
URL:            https://github.com/openSUSE/zypper
%if 0%{?suse_version} > 1325
%else
%endif

# required for documentation

# TUI library which is built and shipped with libzypp-devel

License:        GPL-2.0-or-later

%if 0%{?suse_version}
%requires_ge    libzypp
%else
%endif


