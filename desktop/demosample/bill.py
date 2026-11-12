"""Split a restaurant bill between friends, tip included.

    python3 bill.py 84.50 --people 3 --tip 15
"""
import argparse
import sys


def parse_amount(text):
    """An amount such as "12", "12.5" or "$12.50", in cents."""
    cleaned = text.strip().lstrip("$")
    return round(float(cleaned) * 100)


def split(total_cents, people, tip_percent=0):
    """Each person's share of the bill and its tip, in cents."""
    with_tip = total_cents + round(total_cents * tip_percent / 100)
    share = with_tip // people
    return [share] * people


def format_cents(cents):
    return f"${cents // 100}.{cents % 100:02d}"


def main(argv=None):
    parser = argparse.ArgumentParser(description="Split a bill between friends.")
    parser.add_argument("amount", help="the bill, such as 84.50")
    parser.add_argument("--people", type=int, default=2, help="how many pay (default 2)")
    parser.add_argument("--tip", type=float, default=0, help="the tip in percent (default 0)")
    args = parser.parse_args(argv)
    shares = split(parse_amount(args.amount), args.people, args.tip)
    for number, share in enumerate(shares, 1):
        print(f"Person {number}: {format_cents(share)}")
    print(f"Total:    {format_cents(sum(shares))}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
