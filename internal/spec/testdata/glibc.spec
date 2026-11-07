#
# spec file for package glibc
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


# Run with osc --with=fast_build to have a shorter turnaround
# It will avoid building some parts of glibc
%bcond_with    fast_build

%bcond_with snapshot
%bcond_with ringdisabled

%define flavor @BUILD_FLAVOR@%{nil}

# We need to map from flavor to cross-arch, but as we need the
# result in BuildRequires where the build service evaluates, we
# can use only simple RPM expressions, no lua, no shell, no '{expand:'
# expression :-/  Ideally we'd like to just strip the 'cross_' prefix,
# but we can't.  So enumerate the possibilities for now.
%define cross_arch %{cross_cpu}
%if "%flavor" == "cross-aarch64"
%define cross_cpu aarch64
%endif
%if "%flavor" == "cross-hppa"
%define cross_cpu hppa
%endif
%if "%flavor" == "cross-loongarch64"
%define cross_cpu loongarch64
%endif
%if "%flavor" == "cross-riscv64"
%define cross_cpu riscv64
%endif
%if "%flavor" == "cross-s390x"
%define cross_cpu s390x
%endif
%if "%flavor" == "cross-ppc64le"
%define cross_arch ppc64le
%define cross_cpu powerpc64le
%endif
%if "%flavor" == "cross-x86_64"
%define cross_cpu x86_64
%endif

%if 0%{?cross_cpu:1}
%define binutils_os %{cross_cpu}-suse-linux
# use same sysroot as in binutils.spec
%define sysroot %{_prefix}/%{binutils_os}/sys-root
%endif

%if %{suse_version} >= 1550
%bcond_without usrmerged
%else
%bcond_with usrmerged
%endif

%if %{suse_version} >= 1600
%bcond_with nscd
%else
%bcond_without nscd
%endif

%if 0%{?gcc_version} < 13
%define with_gcc 13
%endif

%bcond_with build_all
%define build_main 1
%define build_utils %{with build_all}
%define build_testsuite %{with build_all}
%define build_cross 0
%if "%flavor" == "utils"
%if %{with ringdisabled}
%endif
%define build_main 0
%define build_utils 1
%define build_testsuite 0
%endif
%if "%flavor" == "testsuite"
%if %{with ringdisabled}
%endif
%define build_main 0
%define build_utils 0
%define build_testsuite 1
%endif
%if 0%{?cross_cpu:1}
%define build_main 0
%define build_utils 0
%define build_testsuite 0
%define build_cross 1
%undefine _build_create_debug
%define _enable_debug_packages 0
%if %{suse_version} < 1600
%endif
%endif
%define host_arch %{?cross_cpu}%{!?cross_cpu:%{_target_cpu}}

# Enable support for livepatching.
%define have_livepatching_support 0
%if %{build_cross}
%if "%{cross_arch}" == "x86_64" || "%{cross_arch}" == "ppc64le"
%define have_livepatching_support 1
%endif
%else
%ifarch x86_64 ppc64le
%define have_livepatching_support 1
%endif
%endif

%if %{have_livepatching_support}
%bcond_without livepatching
%else
%bcond_with livepatching
%endif

%if %{suse_version} >= 1600
%define build_libnsl1 0
%else
%define build_libnsl1 1
%endif

%if %{build_main}
%define name_suffix %{nil}
%else
%define name_suffix -%{flavor}-src
%endif

# Nothing outside of glibc must use these symbols
%define __requires_exclude GLIBC_PRIVATE
%define __provides_exclude GLIBC_PRIVATE
%ifarch i686
# For i686 let's only build what's different from i586, so
# no need to build documentation
%define build_profile 1
%define build_locales 1
%define build_html 0
%else
%if %{with fast_build} || %{build_cross} || %{build_utils} && %{without build_all}
%define build_profile 0
%define build_locales 0
%define build_html 0
%else
# Default:
%define build_profile 1
%define build_locales 1
%define build_html 1
%endif
%endif

%define build_variants %{build_main}

%define disable_assert 0
%define enable_stackguard_randomization 1
# glibc requires at least kernel 3.2
# Bump to 4.3 to enable use of direct socketcalls on x86-32 and s390x
%define enablekernel 4.3
# some architectures need a newer kernel
%ifarch riscv64
%define enablekernel 4.15
%endif
%ifarch loongarch64
%define enablekernel 5.19
%endif

# Before 2.29
%define libnsl_archs %ix86 %alpha hppa m68k %mips32 %mips64 %sparc ppc ppc64 ppc64le x86_64 s390 s390x %arm aarch64 riscv64
# Before 2.34
%define libutil_archs %libnsl_archs
# Before 2.35
%define libanl_archs %libutil_archs

Name:           glibc%{name_suffix}
License:        GPL-2.0-or-later AND LGPL-2.1-or-later AND LGPL-2.1-or-later WITH GCC-exception-2.0
Version:        2.44
Release:        0
%if %{without snapshot}
%define git_id c3a3a9808a
%define libversion %version
%else
%define git_id %(echo %version | sed 's/.*\.g//')
%define libversion %(echo %version | sed 's/\.[^.]*\.g.*//')
%endif
URL:            https://www.gnu.org/software/libc/libc.html
BuildRoot:      %{_tmppath}/%{name}-%{version}-build
Source:         https://ftp.gnu.org/pub/gnu/glibc/glibc-%{version}.tar.xz
%if %{without snapshot}
Source1:        https://ftp.gnu.org/pub/gnu/glibc/glibc-%{version}.tar.xz.sig
%endif
Source2:        https://savannah.gnu.org/project/release-gpgkeys.php?group=libc&download=1&file=./glibc.keyring
Source4:        manpages.tar.bz2
Source5:        nsswitch.conf
Source7:        bindresvport.blacklist
Source9:        glibc.rpmlintrc
Source10:       baselibs.conf
# For systemd
Source20:       nscd.conf
Source21:       nscd.service
Source22:       nscd.sysusers

%if %{build_main}
%if %{without nscd}
%endif
%if %{with usrmerged}
# make sure we have post-usrmerge filesystem package
%endif
# bug437293 - handle update from SLES10 on PowerPC
%ifarch ppc64
%endif
%ifarch ppc
%endif
%ifarch armv6hl armv7hl
# The old runtime linker link gets not provided by rpm find.provides, but it exists
%endif
%endif
%if %{build_utils}
%endif
BuildRoot:      %{_tmppath}/%{name}-%{version}-build
%if 0%{?with_gcc:1}
%endif
%if %{build_testsuite}
%ifarch aarch64 %{ix86} x86_64 ppc64le s390x armv7hl riscv64
%endif
%endif
%if %{build_utils}
%endif
%if %{build_cross}
%endif
%if "%flavor" == "i686"
# Sync only this build counter with the main build
#!BcntSyncTag:  glibc
%endif
%if 0%{?gcc_version} < 14
%if "%flavor" == "cross-loongarch64" || "%flavor" == "cross-hppa"
%endif
%endif

###
# Patches are ordered in the following groups:
# Patches that we will never upstream or which have not been looked at: 0-999
# Patches taken from upstream: 1000-2000
# Patches that are going upstream, waiting approval: 2000-3000
###

###
# Patches that upstream will not accept
###

###
# openSUSE specific patches - won't go upstream
###
### openSUSE extensions, configuration
# PATCH-FIX-OPENSUSE Fix path for nscd databases
# PATCH-FIX-OPENSUSE Fix path for nss_db (bnc#753657) - aj@suse.de
# PATCH-FIX-OPENSUSE adjust nscd.conf
# PATCH-FIX-OPENSUSE -- add some extra information to version output - kukuk@suse.de
# PATCH-FIX-OPENSUSE -- Make --no-archive default for localedef - kukuk@suse.de
# PATCH-FIX-OPENSUSE -- add blacklist for bindresvport
# PATCH-FEATURE-SLE Use nscd user for nscd
# PATCH-FEATURE-SLE read nsswich.conf from /usr
# PATCH-FEATURE-SLE Add ULP prologue into ASM functions

### Locale related patches
# PATCH-FIX-OPENSUSE Add additional locales
# PATCH-FIX-OPENSUSE -- Add no_NO back (XXX: Still needed?)
# PATCH-FIX-OPENSUSE -- Renames for China

### Network related patches
# PATCH-FIX-OPENSUSE Warn about usage of mdns in resolv.conv
# PATCH-FIX-OPENSUSE disable rewriting ::1 to 127.0.0.1 for /etc/hosts bnc#684534, bnc#706719

%if %{without snapshot}
###
# Patches from upstream
###
# PATCH-FIX-UPSTREAM Makerules: Make the .dt to .d conversion safe against concurrent sub-makes
# PATCH-FIX-UPSTREAM math: Fix sinh worst-case results for |x| > 36.736801 (BZ #34441)
# PATCH-FIX-UPSTREAM math: Fix x86_64 tanh _FloatN aliases binding to the FMA variant (BZ #34465)
# PATCH-FIX-UPSTREAM elf: Honour skip_ifunc for cross-object IFUNC relocations (BZ #34428)
# PATCH-FIX-UPSTREAM stdlib: Fix right-justification in strfmon (CVE-2026-19499, BZ #34510)
# PATCH-FIX-UPSTREAM misc: Fix out-of-bounds array write in tdelete (CVE-2026-19542, BZ #34506)
# PATCH-FIX-UPSTREAM iconvdata: SHIFT_JISX0213 decoding lacks pending character reset (CVE-2026-77117, BZ #34556)
# PATCH-FIX-UPSTREAM iconvdata: EUC_JISX0213 decoding lacks pending character reset (CVE-2026-80489, BZ #34568)
# PATCH-FIX-UPSTREAM libio: Fix CVE-2026-18374 heap buffer overflow in ccs= handling (CVE-2026-18374, BZ #34574)
# PATCH-FIX-UPSTREAM resolv: Fix assertion failure on search list truncation (CVE-2026-8674, BZ #31026)
# PATCH-FIX-UPSTREAM elf: Open the normalized $ORIGIN rpath in AT_SECURE programs (CVE-2026-86805, BZ #34360)
# PATCH-FIX-UPSTREAM powerpc: Fix one byte overread in strncasecmp (CVE-2026-97399, BZ #34683)
# PATCH-FIX-UPSTREAM realloc: Fix mmap non-mremap reallocation case (BZ #34697)
%endif

###
# Patches awaiting upstream approval
###
# PATCH-FIX-UPSTREAM Avoid concurrency problem in ldconfig (BZ #23973)

# Non-glibc patches
# PATCH-FIX-OPENSUSE Remove debianisms from manpages

