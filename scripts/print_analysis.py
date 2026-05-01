import json, sys

d = json.load(sys.stdin)
a = d["analysis"]

print(f"{'Player':20s} {'Team':7s} {'Xhair%':>7s} {'Far%':>7s} {'AvgAng':>7s} {'MaxSnp':>7s} {'P99Vel':>7s} {'Snaps':>5s} {'S2T':>3s} {'Track':>6s} {'Fitts':>6s}")
print("-" * 100)

for p in a["players"]:
    susp = len(p.get("suspiciousEvents", []))
    print(
        f"{p['username']:20s} {p['team']:7s} "
        f"{p['crosshairOnEnemyRate']:7.4f} "
        f"{p['crosshairOnEnemyFarRate']:7.4f} "
        f"{p['avgMinAngleToEnemy']:7.1f} "
        f"{p['maxAngularSnap']:7.1f} "
        f"{p['p99AngularVelocity']:7.1f} "
        f"{p['snapCount']:5d} "
        f"{p['snapToTargetCount']:3d} "
        f"{p['trackingScore']:6.4f} "
        f"{p['avgApproachCurvature']:6.4f}"
    )

print()
for p in a["players"]:
    if p.get("suspiciousEvents"):
        print(f"\n  {p['username']} suspicious events:")
        for e in p["suspiciousEvents"][:10]:
            print(f"    [{e['severity']:.3f}] {e['type']}: {e['description']}")
    buckets = [b for b in p.get("distanceBuckets", []) if b["samples"] > 0]
    if buckets:
        print(f"\n  {p['username']} distance breakdown:")
        for b in buckets:
            print(f"    {b['minDistance']:5.0f}-{b['maxDistance']:5.0f}u: crosshair_on={b['crosshairOnRate']:.4f} ({b['samples']:4d} samples)")
