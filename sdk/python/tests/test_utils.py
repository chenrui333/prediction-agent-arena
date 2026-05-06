from __future__ import annotations

import unittest

from arena_client import Market, bps_to_probability, clamp_bps, edge_bps, price_for_outcome, probability_to_bps


class UtilityTests(unittest.TestCase):
    def test_clamp_bps(self) -> None:
        self.assertEqual(clamp_bps(-5), 1)
        self.assertEqual(clamp_bps(0), 1)
        self.assertEqual(clamp_bps(5700), 5700)
        self.assertEqual(clamp_bps(10000), 9999)

    def test_probability_conversion(self) -> None:
        self.assertEqual(bps_to_probability(5700), 0.57)
        self.assertEqual(probability_to_bps(0.57), 5700)
        self.assertEqual(probability_to_bps(-0.1), 1)
        self.assertEqual(probability_to_bps(1.5), 9999)

    def test_edge_bps(self) -> None:
        self.assertEqual(edge_bps(6400, 5700), 700)
        self.assertEqual(edge_bps(5200, 5700), -500)

    def test_price_for_outcome(self) -> None:
        market = Market(
            id=1,
            venue="fake",
            external_id="fake-1",
            slug="market-1",
            title="Demo",
            category="demo",
            status="open",
            yes_price_bps=5700,
            no_price_bps=4300,
        )

        self.assertEqual(price_for_outcome(market, "yes"), 5700)
        self.assertEqual(price_for_outcome(market, "no"), 4300)
        with self.assertRaisesRegex(ValueError, "yes or no"):
            price_for_outcome(market, "maybe")


if __name__ == "__main__":
    unittest.main()
