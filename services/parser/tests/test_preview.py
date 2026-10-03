import base64
import json
from pathlib import Path
import unittest
from unittest.mock import Mock

import fitz

from app.preview import extract_affiliations, preview_pdf


class PreviewTests(unittest.TestCase):
    def make_pdf(self, affiliations):
        doc = fitz.open()
        page = doc.new_page()
        page.insert_text((50, 50), "University Models for Research", fontsize=18)
        page.insert_text((50, 80), "Alice Smith, Bob Jones", fontsize=11)
        y = 100
        for affiliation in affiliations:
            page.insert_text((50, y), affiliation, fontsize=10)
            y += 15
        page.insert_text((50, y + 15), "Abstract", fontsize=12)
        page.insert_text((50, y + 35), "We compare results from Other University.", fontsize=10)
        return doc

    def test_header_only_and_deduplication(self):
        with self.make_pdf([
            "1 Department of Computing, Stanford University, USA",
            "2 Massachusetts Institute of Technology",
            "Stanford University",
            "contact@university.edu",
        ]) as doc:
            self.assertEqual(extract_affiliations(doc[0]), [
                "Stanford University", "Massachusetts Institute of Technology",
            ])

    def test_missing_affiliations(self):
        with self.make_pdf(["Independent Researcher"]) as doc:
            self.assertEqual(extract_affiliations(doc[0]), [])

    def test_abbreviated_university(self):
        with self.make_pdf(["1 UC Berkeley, 2 Physical Intelligence", "3 MIT; 4 EPFL"]) as doc:
            self.assertEqual(extract_affiliations(doc[0]), ["UC Berkeley", "Physical Intelligence", "MIT", "EPFL"])

    def test_companies_and_five_organization_limit(self):
        with self.make_pdf([
            "1 Google DeepMind; 2 OpenAI; 3 Anthropic",
            "4 NVIDIA; 5 Microsoft Research; 6 Meta AI",
        ]) as doc:
            self.assertEqual(extract_affiliations(doc[0]), [
                "Google DeepMind", "OpenAI", "Anthropic", "NVIDIA", "Microsoft Research",
            ])

    def test_numbered_unknown_company_next_to_university(self):
        with self.make_pdf(["1 Stanford University, 2 Example Robotics", "USA"]) as doc:
            self.assertEqual(extract_affiliations(doc[0]), ["Stanford University", "Example Robotics"])

    def test_real_qgf_affiliations(self):
        pdf = Path(__file__).resolve().parents[3] / "frontend/src/shared/assets/qgf-flow-policies.pdf"
        with fitz.open(pdf) as doc:
            self.assertEqual(extract_affiliations(doc[0]), ["UC Berkeley", "Physical Intelligence"])

    def test_real_author_columns_keep_mws_ai(self):
        self.assertEqual(self.extract_header_fixture("gala_header.json"), [
            "Mohamed bin Zayed University of Artificial Intelligence", "MWS AI",
        ])

    def test_real_numbered_columns_keep_amazon_far(self):
        self.assertEqual(self.extract_header_fixture("rpg_header.json"), [
            "UC Berkeley", "Amazon FAR", "MIT", "University of Chicago",
        ])

    def extract_header_fixture(self, name):
        # Снимки текстовой геометрии настоящих PDF; URL источника сохранён в JSON.
        fixture = json.loads((Path(__file__).parent / "fixtures" / name).read_text())
        page = Mock(spec=fitz.Page)
        page.rect = fitz.Rect(0, 0, fixture["width"], fixture["height"])
        page.get_text.return_value = {"blocks": fixture["blocks"]}
        return extract_affiliations(page)

    def test_parallel_address_and_department_are_not_companies(self):
        with self.make_pdf([]) as doc:
            page = doc[0]
            page.insert_text((50, 100), "Stanford University", fontsize=10)
            page.insert_text((250, 100), "Department of Computing", fontsize=10)
            page.insert_text((450, 100), "USA", fontsize=10)
            self.assertEqual(extract_affiliations(page), ["Stanford University"])

    def test_caption_stops_affiliation_scan(self):
        with self.make_pdf([]) as doc:
            page = doc[0]
            page.insert_text((50, 90), "Figure 1: Teaser", fontsize=10)
            page.insert_text((50, 100), "Stanford University", fontsize=10)
            self.assertEqual(extract_affiliations(page), [])

    def test_preview_is_a_bounded_png(self):
        with self.make_pdf(["Peking University"]) as doc:
            result = preview_pdf(doc.tobytes())
        image = base64.b64decode(result["image"].split(",", 1)[1])
        self.assertTrue(image.startswith(b"\x89PNG"))
        pix = fitz.Pixmap(image)
        self.assertLessEqual(pix.width, 1201)
        self.assertLessEqual(pix.height, 1801)
        self.assertEqual(result["affiliations"], ["Peking University"])

    def test_invalid_document(self):
        with self.assertRaises(Exception):
            preview_pdf(b"not a PDF")


if __name__ == "__main__":
    unittest.main()
