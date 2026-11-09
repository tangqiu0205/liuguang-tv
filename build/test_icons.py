import unittest
import xml.etree.ElementTree as ET
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class IconPipelineTests(unittest.TestCase):
    def test_svg_is_self_contained_and_square(self):
        svg = ET.parse(ROOT / "assets/icon.svg").getroot()
        self.assertEqual(svg.attrib["viewBox"], "0 0 512 512")
        self.assertEqual(svg.attrib["width"], svg.attrib["height"])
        for element in svg.iter():
            self.assertNotIn(element.tag.split("}")[-1], ("script", "image", "text"))
            for key, value in element.attrib.items():
                if key.endswith("href"):
                    self.assertTrue(value.startswith("#"))

    def test_icons_are_generated_by_reusable_workflow(self):
        workflow = (ROOT / ".github/workflows/icons.yml").read_text(encoding="utf-8")
        for required in ("workflow_dispatch:", "workflow_call:", "inputs.ref || github.ref",
                         "build/icon-requirements.txt", "python build/generate_icons.py",
                         "name: app-icons", "dist/icons/*.ico", "dist/icons/*.png"):
            self.assertIn(required, workflow)

    def test_release_embeds_icon_before_build_without_replacing_manifest(self):
        workflow = (ROOT / ".github/workflows/release.yml").read_text(encoding="utf-8")
        for required in ("uses: ./.github/workflows/icons.yml", "needs: metadata",
                         "name: app-icons", ".icon-artifacts/dist/icons/liuguangtv.ico",
                         "github.com/tc-hib/go-winres@v0.3.3", "--manifest none",
                         "--out cmd/liuguangtv/rsrc", "rsrc_windows_$arch.syso"):
            self.assertIn(required, workflow)
        self.assertLess(workflow.index("name: Embed Windows application icon"),
                        workflow.index("name: Build packages and signed updates"))


if __name__ == "__main__":
    unittest.main()
