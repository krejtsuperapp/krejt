import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../services/location.dart';
import '../../state/app_state.dart';
import '../account/addresses.dart';
import 'quote.dart';

/// Një pikë e zgjedhur nga përdoruesi, me emrin që i shfaqet.
class Place {
  const Place({required this.point, required this.label});

  final LatLng point;
  final String label;
}

/// Zgjedhja e nisjes dhe e destinacionit. Nisja merret nga pajisja; destinacioni zgjidhet
/// nga adresat e ruajtura ose nga destinacionet e fundit — pa shkrim të lirë, sepse
/// një adresë pa koordinata nuk i shërben as çmimit as shoferit (§17).
class DestinationScreen extends StatefulWidget {
  const DestinationScreen({super.key});

  @override
  State<DestinationScreen> createState() => _DestinationScreenState();
}

class _DestinationScreenState extends State<DestinationScreen> {
  static const _location = LocationService();

  Place? _pickup;
  Place? _dropoff;
  List<Address> _addresses = const [];
  bool _locating = true;
  String? _locationError;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) {
      _locate();
      _loadAddresses();
    });
  }

  Future<void> _locate() async {
    if (mounted) setState(() => _locating = true);
    final result = await _location.current();
    if (!mounted) return;
    setState(() {
      _locating = false;
      if (result.isOk) {
        _pickup = Place(point: result.point!, label: context.t('location.use_current'));
        _locationError = null;
      } else {
        _locationError = context.t(locationProblemKey(result.problem!));
      }
    });
  }

  Future<void> _loadAddresses() async {
    try {
      final items = await context.read<AppState>().api.addresses();
      if (mounted) setState(() => _addresses = items);
    } on ApiError {
      // Adresat mungojnë; destinacionet e fundit mbeten si rrugë e dytë.
    }
  }

  /// Destinacionet e fundit dalin nga historiku: adresa e zbritjes e udhëtimeve të mbyllura.
  List<Place> get _recent {
    final seen = <String>{};
    final out = <Place>[];
    for (final r in context.read<AppState>().recentRides) {
      final label = r.dropoffAddress;
      if (label == null || label.isEmpty || !seen.add(label)) continue;
      out.add(Place(point: r.dropoff, label: label));
      if (out.length == 4) break;
    }
    return out;
  }

  Future<void> _continue() async {
    final pickup = _pickup;
    final dropoff = _dropoff;
    if (pickup == null || dropoff == null) return;
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => QuoteScreen(pickup: pickup, dropoff: dropoff),
      ),
    );
  }

  @override
  Widget build(BuildContext context) {
    final pickup = _pickup;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('home.services.ride'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            KSectionHeader(context.t('ride.pickup.choose')),
            const SizedBox(height: K.s3),
            if (_locating)
              const KSkeleton(height: 64, count: 1)
            else if (pickup != null)
              KCard(
                highlight: true,
                child: Row(
                  children: [
                    const Icon(Icons.my_location, size: 20, color: K.brand400),
                    const SizedBox(width: K.s3),
                    Expanded(
                      child: Text(
                        pickup.label,
                        style: const TextStyle(
                          fontSize: 15,
                          fontWeight: FontWeight.w600,
                          color: K.text,
                        ),
                      ),
                    ),
                    IconButton(
                      icon: const Icon(Icons.refresh, size: 20, color: K.muted),
                      tooltip: context.t('common.retry'),
                      onPressed: _locate,
                    ),
                  ],
                ),
              )
            else
              KError(
                message: _locationError ?? context.t('location.failed'),
                retryLabel: context.t('common.retry'),
                onRetry: _locate,
                icon: Icons.location_off_outlined,
              ),
            const SizedBox(height: K.s6),
            KSectionHeader(context.t('ride.dropoff.choose')),
            const SizedBox(height: K.s3),
            if (_addresses.isEmpty && _recent.isEmpty)
              KEmpty(
                title: context.t('account.address.empty'),
                message: context.t('account.address.empty.hint'),
                icon: Icons.place_outlined,
                action: context.t('account.address.add'),
                onAction: () async {
                  await Navigator.of(context)
                      .push<bool>(MaterialPageRoute(builder: (_) => const AddAddressScreen()));
                  await _loadAddresses();
                },
              ),
            for (final a in _addresses)
              _PlaceRow(
                icon: addressIcon(a.label),
                title: a.name ?? context.t(addressLabelKey(a.label)),
                subtitle: '${a.line1}, ${a.city}',
                selected: _dropoff?.label == a.line1,
                onTap: () =>
                    setState(() => _dropoff = Place(point: LatLng(a.lat, a.lng), label: a.line1)),
              ),
            if (_recent.isNotEmpty) ...[
              const SizedBox(height: K.s4),
              KSectionHeader(context.t('ride.recent')),
              const SizedBox(height: K.s3),
              for (final p in _recent)
                _PlaceRow(
                  icon: Icons.history,
                  title: p.label,
                  selected: _dropoff?.label == p.label,
                  onTap: () => setState(() => _dropoff = p),
                ),
            ],
            const SizedBox(height: K.s6),
            KButton(
              label: context.t('common.continue'),
              onPressed: (_pickup != null && _dropoff != null) ? _continue : null,
            ),
          ],
        ),
      ),
    );
  }
}

class _PlaceRow extends StatelessWidget {
  const _PlaceRow({
    required this.icon,
    required this.title,
    required this.selected,
    required this.onTap,
    this.subtitle,
  });

  final IconData icon;
  final String title;
  final String? subtitle;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Padding(
    padding: const EdgeInsets.only(bottom: K.s2),
    child: KCard(
      onTap: onTap,
      highlight: selected,
      child: Row(
        children: [
          Icon(icon, size: 20, color: selected ? K.brand400 : K.muted),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: TextStyle(
                    fontSize: 15,
                    fontWeight: FontWeight.w600,
                    color: selected ? K.text : K.textDim,
                  ),
                ),
                if (subtitle != null)
                  Padding(
                    padding: const EdgeInsets.only(top: 2),
                    child: Text(
                      subtitle!,
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                  ),
              ],
            ),
          ),
          if (selected) const Icon(Icons.check_circle, size: 20, color: K.brand400),
        ],
      ),
    ),
  );
}
