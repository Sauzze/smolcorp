import unittest

from bill import format_cents, split


class SplitTests(unittest.TestCase):
    def test_an_even_split(self):
        self.assertEqual(split(1000, 2), [500, 500])

    def test_the_tip_is_shared(self):
        self.assertEqual(split(1000, 2, tip_percent=10), [550, 550])


class FormatTests(unittest.TestCase):
    def test_cents_are_shown_with_two_digits(self):
        self.assertEqual(format_cents(1205), "$12.05")


if __name__ == "__main__":
    unittest.main()
