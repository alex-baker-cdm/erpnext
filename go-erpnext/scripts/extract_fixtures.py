#!/usr/bin/env python3
"""Extract test fixtures from ERPNext Python tests as JSON for Go test harness.

This script is intended to be run inside an ERPNext Docker container where the
ERPNext Python packages are available. It imports the actual Python classes,
runs operations, captures results, and outputs JSON fixture files that the Go
test harness can consume.

Usage (inside ERPNext container):
    python extract_fixtures.py --module valuation --output ../testdata/
    python extract_fixtures.py --module payment_terms --output ../testdata/
    python extract_fixtures.py --module all --output ../testdata/
"""
import json
import argparse
import sys
from pathlib import Path


def extract_valuation_fixtures(output_dir: Path):
    """Extract FIFO/LIFO test fixtures from ERPNext's valuation classes."""
    try:
        from erpnext.stock.valuation import FIFOValuation, LIFOValuation
    except ImportError:
        print(
            "ERROR: Could not import ERPNext valuation classes. "
            "Make sure you are running this inside the ERPNext container.",
            file=sys.stderr,
        )
        sys.exit(1)

    # --- FIFO fixtures ---
    fifo_fixtures = []

    # simple_addition
    v = FIFOValuation([])
    v.add_stock(qty=1, rate=10)
    fifo_fixtures.append(
        {
            "name": "simple_addition",
            "input": {"operations": [{"action": "add", "qty": 1, "rate": 10}]},
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    # simple_removal
    v = FIFOValuation([])
    v.add_stock(qty=1, rate=10)
    v.remove_stock(qty=1)
    fifo_fixtures.append(
        {
            "name": "simple_removal",
            "input": {
                "operations": [
                    {"action": "add", "qty": 1, "rate": 10},
                    {"action": "remove", "qty": 1},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    # merge_same_rate
    v = FIFOValuation([])
    v.add_stock(qty=1, rate=10)
    v.add_stock(qty=1, rate=10)
    fifo_fixtures.append(
        {
            "name": "merge_same_rate",
            "input": {
                "operations": [
                    {"action": "add", "qty": 1, "rate": 10},
                    {"action": "add", "qty": 1, "rate": 10},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    # negative_stock
    v = FIFOValuation([])
    v.remove_stock(qty=1, outgoing_rate=5)
    fifo_fixtures.append(
        {
            "name": "negative_stock",
            "input": {
                "initial_state": [],
                "operations": [
                    {"action": "remove", "qty": 1, "outgoing_rate": 5}
                ],
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    # remove_specified_rate
    v = FIFOValuation([])
    v.add_stock(qty=1, rate=10)
    v.add_stock(qty=1, rate=20)
    v.remove_stock(qty=1, outgoing_rate=20)
    fifo_fixtures.append(
        {
            "name": "remove_specified_rate",
            "input": {
                "operations": [
                    {"action": "add", "qty": 1, "rate": 10},
                    {"action": "add", "qty": 1, "rate": 20},
                    {"action": "remove", "qty": 1, "outgoing_rate": 20},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    # remove_multiple_bins
    v = FIFOValuation([])
    v.add_stock(qty=1, rate=10)
    v.add_stock(qty=2, rate=20)
    v.add_stock(qty=1, rate=20)
    v.add_stock(qty=5, rate=20)
    v.remove_stock(qty=4)
    fifo_fixtures.append(
        {
            "name": "remove_multiple_bins",
            "input": {
                "operations": [
                    {"action": "add", "qty": 1, "rate": 10},
                    {"action": "add", "qty": 2, "rate": 20},
                    {"action": "add", "qty": 1, "rate": 20},
                    {"action": "add", "qty": 5, "rate": 20},
                    {"action": "remove", "qty": 4},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
            },
        }
    )

    fifo_file = {
        "module": "stock.valuation",
        "function": "FIFOValuation",
        "fixtures": fifo_fixtures,
    }
    output_path = output_dir / "valuation_fifo.json"
    output_path.write_text(json.dumps(fifo_file, indent=4) + "\n")
    print(f"Wrote {len(fifo_fixtures)} FIFO fixtures to {output_path}")

    # --- LIFO fixtures ---
    lifo_fixtures = []

    # lifo_consumption
    v = LIFOValuation([])
    v.add_stock(qty=10, rate=10)
    v.add_stock(qty=10, rate=20)
    consumed = v.remove_stock(qty=15)
    lifo_fixtures.append(
        {
            "name": "lifo_consumption",
            "input": {
                "operations": [
                    {"action": "add", "qty": 10, "rate": 10},
                    {"action": "add", "qty": 10, "rate": 20},
                    {"action": "remove", "qty": 15},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
                "last_consumed": [list(b) for b in consumed],
            },
        }
    )

    # lifo_going_negative
    v = LIFOValuation([])
    v.add_stock(qty=10, rate=10)
    v.add_stock(qty=10, rate=20)
    consumed = v.remove_stock(qty=25)
    lifo_fixtures.append(
        {
            "name": "lifo_going_negative",
            "input": {
                "operations": [
                    {"action": "add", "qty": 10, "rate": 10},
                    {"action": "add", "qty": 10, "rate": 20},
                    {"action": "remove", "qty": 25},
                ]
            },
            "expected": {
                "state": [list(b) for b in v.state],
                "total_qty": float(v.total_qty),
                "total_value": float(v.total_value),
                "last_consumed": [list(b) for b in consumed],
            },
        }
    )

    lifo_file = {
        "module": "stock.valuation",
        "function": "LIFOValuation",
        "fixtures": lifo_fixtures,
    }
    output_path = output_dir / "valuation_lifo.json"
    output_path.write_text(json.dumps(lifo_file, indent=4) + "\n")
    print(f"Wrote {len(lifo_fixtures)} LIFO fixtures to {output_path}")


def extract_payment_term_fixtures(output_dir: Path):
    """Extract payment term due-date fixtures from ERPNext.

    The actual get_due_date(term, posting_date, bill_date) function expects
    a term object with attributes like due_date_based_on, credit_days, etc.
    We use a SimpleNamespace to build lightweight term objects.
    """
    try:
        from erpnext.controllers.accounts_controller import get_due_date
    except ImportError:
        print(
            "ERROR: Could not import ERPNext accounts controller. "
            "Make sure you are running this inside the ERPNext container.",
            file=sys.stderr,
        )
        sys.exit(1)

    import datetime
    from types import SimpleNamespace

    fixtures = []

    # days_after_invoice
    term = SimpleNamespace(
        due_date_based_on="Day(s) after invoice date",
        credit_days=30,
        credit_months=0,
    )
    result = get_due_date(term, posting_date=datetime.date(2024, 1, 15), bill_date=None)
    fixtures.append(
        {
            "name": "days_after_invoice",
            "input": {
                "due_date_based_on": "Day(s) after invoice date",
                "credit_days": 30,
                "posting_date": "2024-01-15",
            },
            "expected": {"due_date": str(result)},
        }
    )

    # days_after_end_of_month
    term = SimpleNamespace(
        due_date_based_on="Day(s) after the end of the invoice month",
        credit_days=15,
        credit_months=0,
    )
    result = get_due_date(term, posting_date=datetime.date(2024, 1, 15), bill_date=None)
    fixtures.append(
        {
            "name": "days_after_end_of_month",
            "input": {
                "due_date_based_on": "Day(s) after the end of the invoice month",
                "credit_days": 15,
                "posting_date": "2024-01-15",
            },
            "expected": {"due_date": str(result)},
        }
    )

    # months_after_end_of_month
    term = SimpleNamespace(
        due_date_based_on="Month(s) after the end of the invoice month",
        credit_days=0,
        credit_months=2,
    )
    result = get_due_date(term, posting_date=datetime.date(2024, 1, 15), bill_date=None)
    fixtures.append(
        {
            "name": "months_after_end_of_month",
            "input": {
                "due_date_based_on": "Month(s) after the end of the invoice month",
                "credit_months": 2,
                "posting_date": "2024-01-15",
            },
            "expected": {"due_date": str(result)},
        }
    )

    payment_file = {
        "module": "controllers.accounts_controller",
        "function": "get_due_date",
        "fixtures": fixtures,
    }
    output_path = output_dir / "payment_terms.json"
    output_path.write_text(json.dumps(payment_file, indent=4) + "\n")
    print(f"Wrote {len(fixtures)} payment term fixtures to {output_path}")


EXTRACTORS = {
    "valuation": extract_valuation_fixtures,
    "payment_terms": extract_payment_term_fixtures,
}


def main():
    parser = argparse.ArgumentParser(
        description="Extract ERPNext test fixtures as JSON for the Go test harness."
    )
    parser.add_argument(
        "--module",
        choices=list(EXTRACTORS.keys()) + ["all"],
        required=True,
        help="Which module's fixtures to extract.",
    )
    parser.add_argument(
        "--output",
        default="testdata/",
        help="Output directory for JSON fixture files (default: testdata/).",
    )
    args = parser.parse_args()

    output_dir = Path(args.output)
    output_dir.mkdir(parents=True, exist_ok=True)

    if args.module == "all":
        for name, extractor in EXTRACTORS.items():
            print(f"\n--- Extracting {name} fixtures ---")
            extractor(output_dir)
    else:
        EXTRACTORS[args.module](output_dir)

    print("\nDone.")


if __name__ == "__main__":
    main()
