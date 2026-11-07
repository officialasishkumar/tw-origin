#
# spec file for package openssl-3
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


%define ssletcdir %{_sysconfdir}/ssl
%define sover 3
%define _rname openssl
%define man_suffix 3ssl

%bcond_with lto
%if %{without lto}
%define _lto_cflags %{nil}
%endif

%if 0%{?suse_version} >= 1550 || 0%{?sle_version} >= 150400
%global sle_needs_crypto_policies 1
%endif

%if 0%{?suse_version} >= 1699
%global openssl_test_flags HARNESS_JOBS=${RPM_BUILD_NCPUS}
%endif

# Enable userspace livepatching.
%define livepatchable 1

Name:           openssl-3
Version:        3.5.3
Release:        0
License:        Apache-2.0
URL:            https://www.openssl.org/
Source:         https://www.%{_rname}.org/source/%{_rname}-%{version}.tar.gz
Source1:        https://www.%{_rname}.org/source/%{_rname}-%{version}.tar.gz.asc
# https://keys.openpgp.org/search?q=openssl@openssl.org
# BA54 73A2 B058 7B07 FB27 CF2D 2160 94DF D0CB 81EF
Source2:        %{_rname}.keyring
# to get mtime of file:
Source3:        %{name}.changes
Source4:        baselibs.conf
Source5:        openssl-TESTS-Disable-default-provider-crypto-policies.patch
# PATCH-FIX-OPENSUSE: Do not install html docs as it takes ages
# PATCH-FIX-FEDORA Add crypto-policies support
# PATCH-FIX-FEDORA Add FIPS_mode compatibility macro and flag support
# PATCH-FIX-FEDORA Load FIPS the provider and set FIPS properties implicitly
# PATCH-FIX-FEDORA Disable the fipsinstall command-line utility
# PATCH-FIX-FEDORA Instructions to load legacy provider in openssl.cnf
# PATCH-FIX-FEDORA Embed the FIPS hmac
# PATCH-FIX-FEDORA bsc#1221786 FIPS: Use of non-Approved Elliptic Curves
# PATCH-FIX-FEDORA bsc#1221753 bsc#1221760 bsc#1221822 FIPS: Extra public/private key checks required by FIPS-140-3
# PATCH-FIX-FEDORA bsc#1221760 FIPS: Execute KATS before HMAC verification
# PATCH-FIX-FEDORA bsc#1221365 bsc#1221824 FIPS: Service Level Indicator is needed
# PATCH-FIX-FEDORA bsc#1221760 FIPS: Execute KATS before HMAC verification
# PATCH-FIX-FEDORA bsc#1221365 bsc#1221365 FIPS: Service Level Indicator is needed
# PATCH-FIX-FEDORA bsc#1221827 FIPS: Recommendation for Password-Based Key Derivation
# PATCH-FIX-FEDORA bsc#1221365 FIPS: Service Level Indicator is needed
# PATCH-FIX-FEDORA bsc#1221824 FIPS: NIST SP 800-56Brev2 Section 6.4.1.2.1
# PATCH-FIX-FEDORA bsc#1221821 FIPS: Disable FIPS 186-4 Domain Parameters
# PATCH-FIX-SUSE bsc#1221824 FIPS: Add check for SP 800-56Brev2 Section 6.4.1.2.1
# PATCH-FIX-SUSE bsc#1221753 FIPS: Enforce error state
# PATCH-FIX-FEDORA Adapt pairwise tests
# PATCH-FIX-FEDORA Fix broken selftests in fips provider init
# PATCH-FIX-SUSE Use the shared jitterentropy library instead of static
# PATCH-FIX-SUSE Disable dubious broken test
# PATCH-FIX-FEDORA bsc#1221365 FIPS: Service Level Indicator is needed
# PATCH-FIX-FEDORA bsc#1221787 FIPS: Selectively disallow SHA1 signatures
# PATCH-FIX-FEDORA bsc#1221365 FIPS: Deny SHA-1 signature verification in FIPS provider
# PATCH-FIX-FEDORA FIPS: Allow SHA1 in seclevel 2 if rh-allow-sha1-signatures = yes
# PATCH-FIX-FEDORA FIPS: Fix the speed command in FIPS mode for KMAC
# PATCH-FIX-SUSE Fix a bogus warning caused by -Wfree-nonheap-object
# PATCH-FIX-UPSTREAM bsc#1250232 CVE-2025-9230: Fix out-of-bounds read & write in RFC 3211 KEK Unwrap
# PATCH-FIX-UPSTREAM bsc#1250233 CVE-2025-9231: Fix timing side-channel in SM2 algorithm on 64 bit ARM
# PATCH-FIX-UPSTREAM bsc#1250234 CVE-2025-9232: Fix out-of-bounds read in HTTP client no_proxy handling
# PATCH-FIX-UPSTREAM bsc#1256839 CVE-2026-22795: Missing ASN1_TYPE validation in PKCS#12 parsing
# PATCH-FIX-UPSTREAM bsc#1256840 CVE-2026-22796: ASN1_TYPE Type Confusion in the PKCS7_digest_from_attributes() function
# PATCH-FIX-UPSTREAM bsc#1256837 CVE-2025-69420: Missing ASN1_TYPE validation in TS_RESP_verify_response() function
# PATCH-FIX-UPSTREAM bsc#1256838 CVE-2025-69421: NULL Pointer Dereference in PKCS12_item_decrypt_d2i_ex function
# PATCH-FIX-UPSTREAM bsc#1256836 CVE-2025-69419: Out of bounds write in PKCS12_get_friendlyname() UTF-8 function
# PATCH-FIX-UPSTREAM bsc#1256833 CVE-2025-66199: TLS 1.3 CompressedCertificate excessive memory allocation
# PATCH-FIX-UPSTREAM bsc#1256834 CVE-2025-68160: Heap out-of-bounds write in BIO_f_linebuffer on short writes
# PATCH-FIX-UPSTREAM bsc#1256835 CVE-2025-69418: Unauthenticated/unencrypted trailing bytes with low-level OCB function calls
# PATCH-FIX-UPSTREAM bsc#1256832 CVE-2025-15469: 'openssl dgst' one-shot codepath silently truncates inputs greater than 16MB
# PATCH-FIX-UPSTREAM bsc#1256830 CVE-2025-15467: Stack buffer overflow in CMS AuthEnvelopedData parsing
# PATCH-FIX-UPSTREAM bsc#1256829 CVE-2025-11187: Improper validation of PBMAC1 parameters in PKCS#12 MAC verification
# PATCH-FIX-UPSTREAM bsc#1256831 CVE-2025-15468: NULL dereference in SSL_CIPHER_find() function on unknown cipher ID
# PATCH-FIX_UPSTREAM bsc#1259652 CVE-2026-2673: TLS 1.3 servers may choose unexpected key agreement group
# PATCH-FIX-UPSTREAM CVE-2026-28387: Potential use-after-free in DANE client code (bsc#1260441)
# PATCH-FIX-UPSTREAM CVE-2026-28388: NULL Pointer Dereference When Processing a Delta (bsc#1260442)
# PATCH-FIX-UPSTREAM CVE-2026-28389: Possible NULL dereference when processing CMS KeyAgreeRecipientInfo (bsc#1260443)
# PATCH-FIX-UPSTREAM CVE-2026-31789: Heap buffer overflow in hexadecimal conversion (bsc#1260444)
# PATCH-FIX-UPSTREAM CVE-2026-31790: Incorrect failure handling in RSA KEM RSASVE encapsulation (bsc#1260445)
# PATCH-FIX-UPSTREAM: NULL pointer dereference when processing an OCSP response
# PATCH-FIX-UPSTREAM CVE-2026-28390: NULL pointer dereference during processing of a crafted CMS EnvelopedData message with KeyTransportRecipientInfo (bsc#1261678)
# PATCH-FIX-UPSTREAM: Optimized MLKEM NTT, supports p8 (ISA 2.07) and above architectures (jsc#PED-14569)
# PATCH-FIX-UPSTREAM: CVE-2026-45447 Heap Use-After-Free in OpenSSL PKCS7_verify() (bsc#1266357)
# PATCH-FIX-UPSTREAM: CVE-2026-45446 Incorrect Tag Processing for Empty Messages in AES-GCM-SIV and AES-SIV modes (bsc#1266356)
# PATCH-FIX-UPSTREAM: CVE-2026-42770 FFC-DH Peer Validation Uses Attacker-Supplied q (bsc#1266353)
# PATCH-FIX-UPSTREAM: CVE-2026-45445 AES-OCB IV Ignored on EVP_Cipher() Path (bsc#1266355)
# PATCH-FIX-UPSTREAM: CVE-2026-42767 NULL Pointer Dereference in CRMF EncryptedValue Decryption (bsc#1266350)
# PATCH-FIX-UPSTREAM: CVE-2026-42768 Multi-RecipientInfo Bleichenbacher Oracle in CMS_decrypt() and PKCS7_decrypt() (bsc#1266351)
# PATCH-FIX-UPSTREAM: CVE-2026-42769 Trust-Anchor Substitution via cert/issuer Typo in CMP rootCaKeyUpdate (bsc#1266352)
# PATCH-FIX-UPSTREAM: CVE-2026-42766 Possible NULL Dereference in Password-Based CMS Decryption (bsc#1266349)
# PATCH-FIX-UPSTREAM: CVE-2026-34183 Unbounded Memory Growth in the QUIC PATH_CHALLENGE Handler (bsc#1266345)
# PATCH-FIX-UPSTREAM: CVE-2026-42764 NULL pointer dereference in QUIC server initial packet handling (bsc#1266347)
# PATCH-FIX-UPSTREAM: CVE-2026-34182 CMS AuthEnvelopedData Processing May Accept Forged Messages (bsc#1266344)
# PATCH-FIX-UPSTREAM: CVE-2026-9076 Out-of-Bounds Read in CMS Password-Based Decryption (bsc#1266341)
# PATCH-FIX-UPSTREAM: CVE-2026-7383 Possible Heap Buffer Overflow in ASN.1 Multibyte String Conversion (bsc#1266340)
# PATCH-FIX-UPSTREAM: CVE-2026-34180 Heap Buffer Over-read in ASN.1 Content Parsing (bsc#1266342)
# PATCH-FIX-UPSTREAM: Grow the init_buf incrementally as we receive data (bsc#1271712)
# PATCH-FIX-UPSTREAM: CVE-2026-14456: Unbounded Memory Growth in QUIC Server Incoming Channel Queue (bsc#1274791)
# PATCH-FIX-UPSTREAM: CVE-2026-14457: RPK Server Signature Algorithm Selection Can Dereference a Missing Certificate (bsc#1274792)
# PATCH-FIX-UPSTREAM: CVE-2026-18798: QUIC Server May Trigger Double Free When Processing INITIAL Packet (bsc#1274777)
# PATCH-FIX-UPSTREAM: CVE-2026-34181: PKCS#12 Files with PBMAC1 Are Accepted with Short HMAC Keys (bsc#1266343)
# PATCH-FIX-UPSTREAM: CVE-2026-54874: Excessive Memory Use Buffering DTLS Records for a Future Epoch (bsc#1274795)
# PATCH-FIX-UPSTREAM: CVE-2026-63072: Heap Buffer Overflow in CMS Key Unwrapping (bsc#1274788)
# PATCH-FIX-UPSTREAM: CVE-2026-63073: Untrusted Sender DN Used as Format String in CMP Response Validation (bsc#1274796)
# PATCH-FIX-UPSTREAM: CVE-2026-63074: CMP Indefinite Cache Growth of ExtraCerts (bsc#1274797)
# PATCH-FIX-UPSTREAM: CVE-2026-63075: QUIC ACK-only Packet Retention Can Cause Memory Exhaustion (bsc#1274798)
# PATCH-FIX-UPSTREAM: CVE-2026-63076: Invalid Pointer Dereference in CMP Server via Crafted protectionAlg (bsc#1274790)
# PATCH-FIX-UPSTREAM: CVE-2026-75803: AEAD Forgeries with Empty Ciphertext When Using EVP_Cipher() (bsc#1275837)

# ulp-macros is available according to SUSE version.
%if 0%{?sle_version} >= 150400 || 0%{?suse_version} >= 1540
%endif
# Needed for clean upgrade path, boo#1070003
# Needed for clean upgrade from former openssl-1_1_0, boo#1081335
%{?suse_build_hwcaps_libs}
%if 0%{?sle_needs_crypto_policies}
%endif

