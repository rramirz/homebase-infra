import re
import unittest


PROTECTED_REMOVAL = re.compile(
    r"(?m)^Remv\s+(?:proxmox-ve|pve-manager|proxmox-default-kernel|proxmox-kernel-helper)\b"
)
TARGET_KERNEL_PACKAGE = re.compile(
    r"(?m)^proxmox-kernel-(?:6\.(?:14|17)|(?:[7-9]|[1-9][0-9]+)\.[0-9]+).*install ok installed"
)
TARGET_CONCRETE_KERNEL_PACKAGE = re.compile(
    r"(?m)^proxmox-kernel-((?:6\.(?:14|17)|(?:[7-9]|[1-9][0-9]+)\.[0-9]+)\..*-pve)-signed\s+.*install ok installed"
)
TARGET_KERNEL_RELEASE = re.compile(
    r"^(?:6\.(?:14|17)|(?:[7-9]|[1-9][0-9]+)\.[0-9]+)\..*-pve$"
)
TARGET_META_PACKAGES = re.compile(
    r"(?m)^proxmox-(?:default-kernel|kernel-helper)\s+.*install ok installed"
)
TARGET_PVE9_PROTECTED = re.compile(
    r"(?m)^(?:proxmox-ve|pve-manager)\s+9\..*install ok installed"
)
OFFICIAL_DEBIAN_SOURCE = re.compile(
    r"^\s*deb(?:-src)?\s+(?:\[[^]]+\]\s+)?https?://(?:(?:deb|security)\.debian\.org|ftp\.[^/]+\.debian\.org)/\S*\s+(?P<suite>\S+)\s+.+$"
)
CANONICAL_DEBIAN_SOURCES = """deb http://deb.debian.org/debian trixie main contrib non-free non-free-firmware
deb http://deb.debian.org/debian trixie-updates main contrib non-free non-free-firmware
deb http://security.debian.org/debian-security trixie-security main contrib non-free non-free-firmware
"""
CANONICAL_PVE_SOURCE = "deb http://download.proxmox.com/debian/pve trixie pve-no-subscription\n"
PVE_ENTERPRISE_SOURCES = "/etc/apt/sources.list.d/pve-enterprise.sources"
PVE_NO_SUBSCRIPTION_SOURCE = "/etc/apt/sources.list.d/pve-no-subscription.list"
CANONICAL_ACTIVE_PVE_SOURCE_FILES = [PVE_NO_SUBSCRIPTION_SOURCE]


class ProxmoxUpgradePolicyTests(unittest.TestCase):
    def test_protected_removals_are_detected(self):
        self.assertIsNotNone(PROTECTED_REMOVAL.search("Remv pve-manager [8.4.0]"))
        self.assertIsNotNone(PROTECTED_REMOVAL.search("Remv proxmox-ve [8.4.0]"))
        self.assertIsNotNone(PROTECTED_REMOVAL.search("Remv proxmox-default-kernel [1.0]"))
        self.assertIsNotNone(PROTECTED_REMOVAL.search("Remv proxmox-kernel-helper [8.4.0]"))
        self.assertIsNone(PROTECTED_REMOVAL.search("Inst pve-manager [8.4.0] (9.0.0)"))

    def test_target_kernel_policy_accepts_supported_families_not_pve8_kernel(self):
        for package in (
            "proxmox-kernel-6.14.11-2-pve install ok installed",
            "proxmox-kernel-6.17.4-1-pve install ok installed",
            "proxmox-kernel-7.0 7.0.14-14 install ok installed",
            "proxmox-kernel-7.0.14-14-pve-signed 7.0.14-14 install ok installed",
        ):
            self.assertIsNotNone(TARGET_KERNEL_PACKAGE.search(package))
        self.assertIsNone(TARGET_KERNEL_PACKAGE.search("proxmox-kernel-6.8.12-1-pve install ok installed"))
        self.assertIsNotNone(TARGET_KERNEL_RELEASE.match("6.17.4-1-pve"))
        self.assertIsNotNone(TARGET_KERNEL_RELEASE.match("7.0.14-14-pve"))
        self.assertIsNone(TARGET_KERNEL_RELEASE.match("7.0.14-14-pve-extra"))
        self.assertIsNone(TARGET_KERNEL_RELEASE.match("6.8.12-1-pve"))

    def test_concrete_signed_kernel_policy_rejects_meta_only_state(self):
        signed = "proxmox-kernel-7.0.14-14-pve-signed 7.0.14-14 install ok installed"
        meta_only = "proxmox-kernel-7.0 7.0.14-14 install ok installed"
        concrete = TARGET_CONCRETE_KERNEL_PACKAGE.search(signed)
        if concrete is None:
            self.fail("expected concrete signed target kernel package to match")
        self.assertEqual(concrete.group(1), "7.0.14-14-pve")
        self.assertIsNone(TARGET_CONCRETE_KERNEL_PACKAGE.search(meta_only))
        self.assertEqual(f"/boot/vmlinuz-{concrete.group(1)}", "/boot/vmlinuz-7.0.14-14-pve")

    def test_canonical_source_definition_has_exact_target_suites(self):
        self.assertEqual(CANONICAL_DEBIAN_SOURCES.count("\n"), 3)
        self.assertIn("trixie-updates", CANONICAL_DEBIAN_SOURCES)
        self.assertIn("trixie-security", CANONICAL_DEBIAN_SOURCES)
        self.assertNotIn("bookworm", CANONICAL_DEBIAN_SOURCES)

    def test_routine_source_policy_rejects_wrong_suite(self):
        source = "deb http://ftp.us.debian.org/debian trixie main contrib non-free non-free-firmware"
        match = OFFICIAL_DEBIAN_SOURCE.match(source)
        if match is None:
            self.fail("expected official Debian source format to match")
        self.assertNotIn(match.group("suite"), {"bookworm", "bookworm-updates", "bookworm-security"})

    def test_source_policy_rejects_arbitrary_mirror(self):
        source = "deb https://mirror.example.invalid/debian bookworm main"
        self.assertIsNone(OFFICIAL_DEBIAN_SOURCE.match(source))

    def test_canonical_pve_source_policy_allows_only_no_subscription(self):
        self.assertEqual(CANONICAL_ACTIVE_PVE_SOURCE_FILES, [PVE_NO_SUBSCRIPTION_SOURCE])
        self.assertEqual(
            CANONICAL_PVE_SOURCE,
            "deb http://download.proxmox.com/debian/pve trixie pve-no-subscription\n",
        )

    def test_generated_enterprise_deb822_source_is_explicitly_allowlisted_for_cleanup(self):
        with open("proxmox-major-upgrade.yaml", encoding="utf-8") as playbook:
            content = playbook.read()
        with open("tasks/proxmox-major-source-policy.yaml", encoding="utf-8") as source_policy:
            source_policy_content = source_policy.read()
        self.assertIn(PVE_ENTERPRISE_SOURCES, content)
        self.assertIn(PVE_ENTERPRISE_SOURCES, source_policy_content)
        self.assertIn("cleanup_generated_pve_enterprise_source", source_policy_content)

    def test_source_policy_resets_accumulators_before_each_inclusion(self):
        with open("tasks/proxmox-major-source-policy.yaml", encoding="utf-8") as source_policy:
            content = source_policy.read()
        reset_index = content.index("Reset canonical source policy accumulators")
        for accumulator in (
            "source_policy_unsupported_active_sources: []",
            "source_policy_noncanonical_active_sources: []",
            "source_policy_active_pve_source_files: []",
        ):
            self.assertIn(accumulator, content[: content.index("Check whether generated PVE enterprise")])
        self.assertLess(reset_index, content.index("Identify unsupported active source files"))

    def test_enterprise_source_backup_is_private_and_non_overwriting(self):
        with open("tasks/proxmox-major-source-policy.yaml", encoding="utf-8") as source_policy:
            content = source_policy.read()
        self.assertIn("mode: '0700'", content)
        self.assertIn("mode: '0600'", content)
        self.assertIn("force: false", content)
        self.assertLess(
            content.index("Preserve generated enterprise source before deterministic cleanup"),
            content.index("Disable regenerated PVE enterprise deb822 source deterministically"),
        )

    def test_source_policy_rejects_active_enterprise_source_after_cleanup(self):
        active_pve_source_files = [PVE_NO_SUBSCRIPTION_SOURCE, PVE_ENTERPRISE_SOURCES]
        self.assertNotEqual(active_pve_source_files, CANONICAL_ACTIVE_PVE_SOURCE_FILES)

    def test_target_meta_packages_must_be_installed(self):
        installed = "proxmox-default-kernel 1.0 install ok installed\nproxmox-kernel-helper 9.0 install ok installed"
        self.assertEqual(len(TARGET_META_PACKAGES.findall(installed)), 2)

    def test_target_pve_meta_packages_must_remain_pve9(self):
        installed = "proxmox-ve 9.0.1 install ok installed\npve-manager 9.0.2 install ok installed"
        self.assertEqual(len(TARGET_PVE9_PROTECTED.findall(installed)), 2)

    def test_taskfile_keeps_exact_host_guards(self):
        with open("Taskfile.yaml", encoding="utf-8") as taskfile:
            content = taskfile.read()
        self.assertGreaterEqual(content.count('case "{{.PVE_HOST}}" in pve-1|pve-2)'), 6)


if __name__ == "__main__":
    unittest.main()
