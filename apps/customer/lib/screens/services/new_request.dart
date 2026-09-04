import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';
import '../ride/place_search.dart';
import 'request_tracking.dart';

/// Ikona e kategorisë; e ndajnë ekrani i kërkesës dhe ai i ndjekjes.
IconData serviceIconFor(String category) {
  switch (category) {
    case 'electrician':
      return Icons.electrical_services;
    case 'plumber':
      return Icons.plumbing;
    case 'cleaning':
      return Icons.cleaning_services_outlined;
    case 'ac':
      return Icons.ac_unit;
    case 'appliance':
      return Icons.kitchen_outlined;
    case 'moving':
      return Icons.local_shipping_outlined;
  }
  return Icons.handyman_outlined;
}

/// Kërkesa për mjeshtër: kategoria, çfarë duhet bërë, ku dhe kur. Çmimin nuk e vendos klienti
/// dhe nuk e shpik platforma — vjen nga oferta e mjeshtrit (§22).
class NewServiceRequestScreen extends StatefulWidget {
  const NewServiceRequestScreen({super.key});

  @override
  State<NewServiceRequestScreen> createState() => _NewServiceRequestScreenState();
}

class _NewServiceRequestScreenState extends State<NewServiceRequestScreen> {
  final _title = TextEditingController();
  final _description = TextEditingController();

  List<ServiceCategory> _categories = const [];
  String? _category;
  PickedPlace? _address;
  DateTime? _preferredAt;
  String _paymentMethod = 'cash';
  bool _loading = true;
  bool _sending = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _title.dispose();
    _description.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final items = await context.read<AppState>().api.serviceCategories();
      if (!mounted) return;
      setState(() {
        _categories = items;
        _category ??= items.isEmpty ? null : items.first.id;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _loading = false;
      });
    }
  }

  Future<void> _pickAddress() async {
    final place = await showPlaceSearch(context, title: context.t('service.form.address'));
    if (place == null || !mounted) return;
    setState(() {
      _address = place;
      _error = null;
    });
  }

  Future<void> _pickWhen() async {
    final now = DateTime.now();
    final date = await showDatePicker(
      context: context,
      initialDate: now,
      firstDate: now,
      lastDate: now.add(const Duration(days: 30)),
    );
    if (date == null || !mounted) return;
    final time = await showTimePicker(context: context, initialTime: TimeOfDay.now());
    if (!mounted) return;
    setState(() {
      _preferredAt = time == null
          ? DateTime(date.year, date.month, date.day, 9)
          : DateTime(date.year, date.month, date.day, time.hour, time.minute);
    });
  }

  bool get _valid =>
      _category != null &&
      _title.text.trim().isNotEmpty &&
      _description.text.trim().length >= 10 &&
      _address != null;

  Future<void> _submit() async {
    if (!_valid) {
      setState(() => _error = context.t('errors.validation'));
      return;
    }
    setState(() {
      _sending = true;
      _error = null;
    });
    final state = context.read<AppState>();
    try {
      final request = await state.api.createServiceRequest(
        categoryId: _category!,
        title: _title.text.trim(),
        description: _description.text.trim(),
        addressLine1: _address!.label,
        address: _address!.point,
        paymentMethod: _paymentMethod,
        preferredAt: _preferredAt,
      );
      if (!mounted) return;
      unawaited(state.refreshHome());
      await Navigator.of(context).pushReplacement(
        MaterialPageRoute<void>(builder: (_) => ServiceTrackingScreen(requestId: request.id)),
      );
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _sending = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('service.new'))),
      body: SafeArea(
        child: _loading
            ? const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 72, count: 4))
            : ListView(
                padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
                children: [
                  KSectionHeader(context.t('service.category')),
                  const SizedBox(height: K.s3),
                  Wrap(
                    spacing: K.s2,
                    runSpacing: K.s2,
                    children: [
                      for (final c in _categories)
                        _CategoryChip(
                          icon: serviceIconFor(c.id),
                          label: context.t(c.nameKey),
                          selected: _category == c.id,
                          onTap: () => setState(() {
                            _category = c.id;
                            _error = null;
                          }),
                        ),
                    ],
                  ),
                  const SizedBox(height: K.s5),
                  KField(
                    label: context.t('service.form.title'),
                    controller: _title,
                    hint: context.t('service.form.title.hint'),
                    maxLength: 80,
                    textInputAction: TextInputAction.next,
                    onChanged: (_) => setState(() => _error = null),
                  ),
                  const SizedBox(height: K.s3),
                  KField(
                    label: context.t('service.form.description'),
                    controller: _description,
                    hint: context.t('service.form.description.hint'),
                    maxLines: 4,
                    maxLength: 1000,
                    onChanged: (_) => setState(() => _error = null),
                  ),
                  const SizedBox(height: K.s4),
                  KSectionHeader(context.t('service.form.address')),
                  const SizedBox(height: K.s2),
                  _PickRow(
                    icon: Icons.place_outlined,
                    value: _address?.label,
                    hint: context.t('ride.search.hint'),
                    onTap: _pickAddress,
                  ),
                  const SizedBox(height: K.s3),
                  KSectionHeader(context.t('service.form.when')),
                  const SizedBox(height: K.s2),
                  _PickRow(
                    icon: Icons.schedule,
                    value: _preferredAt == null ? null : _formatWhen(_preferredAt!),
                    hint: context.t('service.form.when.any'),
                    onTap: _pickWhen,
                    onClear: _preferredAt == null
                        ? null
                        : () => setState(() => _preferredAt = null),
                  ),
                  const SizedBox(height: K.s4),
                  KSectionHeader(context.t('ride.payment')),
                  const SizedBox(height: K.s2),
                  Row(
                    children: [
                      Expanded(
                        child: _Choice(
                          icon: Icons.payments_outlined,
                          label: context.t('ride.payment.cash'),
                          selected: _paymentMethod == 'cash',
                          onTap: () => setState(() => _paymentMethod = 'cash'),
                        ),
                      ),
                      const SizedBox(width: K.s3),
                      Expanded(
                        child: _Choice(
                          icon: Icons.account_balance_wallet_outlined,
                          label: context.t('ride.payment.wallet'),
                          selected: _paymentMethod == 'wallet',
                          onTap: () => setState(() => _paymentMethod = 'wallet'),
                        ),
                      ),
                    ],
                  ),
                  if (_error != null) ...[
                    const SizedBox(height: K.s4),
                    Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
                  ],
                  const SizedBox(height: K.s6),
                  KButton(
                    label: context.t('service.form.submit'),
                    icon: Icons.send_outlined,
                    busy: _sending,
                    onPressed: _sending ? null : _submit,
                  ),
                  const SizedBox(height: K.s3),
                  Text(
                    context.t('service.form.hint'),
                    textAlign: TextAlign.center,
                    style: const TextStyle(fontSize: 12, color: K.muted, height: 1.4),
                  ),
                ],
              ),
      ),
    );
  }

  static String _formatWhen(DateTime at) =>
      '${at.day}.${at.month} · ${at.hour.toString().padLeft(2, '0')}:${at.minute.toString().padLeft(2, '0')}';
}

class _CategoryChip extends StatelessWidget {
  const _CategoryChip({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => InkWell(
    onTap: onTap,
    borderRadius: BorderRadius.circular(K.rFull),
    child: Container(
      padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
      decoration: BoxDecoration(
        color: selected ? K.brand500.withValues(alpha: 0.14) : K.surface2,
        borderRadius: BorderRadius.circular(K.rFull),
        border: Border.all(color: selected ? K.brand500 : K.line),
      ),
      child: Row(
        mainAxisSize: MainAxisSize.min,
        children: [
          Icon(icon, size: 18, color: selected ? K.brand400 : K.muted),
          const SizedBox(width: K.s2),
          Text(
            label,
            style: TextStyle(
              fontSize: 14,
              fontWeight: FontWeight.w600,
              color: selected ? K.text : K.textDim,
            ),
          ),
        ],
      ),
    ),
  );
}

class _PickRow extends StatelessWidget {
  const _PickRow({
    required this.icon,
    required this.value,
    required this.hint,
    required this.onTap,
    this.onClear,
  });

  final IconData icon;
  final String? value;
  final String hint;
  final VoidCallback onTap;
  final VoidCallback? onClear;

  @override
  Widget build(BuildContext context) => InkWell(
    onTap: onTap,
    borderRadius: BorderRadius.circular(K.rMd),
    child: Container(
      height: 52,
      padding: const EdgeInsets.symmetric(horizontal: K.s4),
      decoration: BoxDecoration(
        color: K.surface2,
        borderRadius: BorderRadius.circular(K.rMd),
        border: Border.all(color: value == null ? K.line : K.line2),
      ),
      child: Row(
        children: [
          Icon(icon, size: 20, color: K.muted),
          const SizedBox(width: K.s3),
          Expanded(
            child: Text(
              value ?? hint,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 15,
                fontWeight: FontWeight.w600,
                color: value == null ? K.muted : K.text,
              ),
            ),
          ),
          if (onClear != null)
            IconButton(
              icon: const Icon(Icons.close, size: 18, color: K.muted),
              onPressed: onClear,
            ),
        ],
      ),
    ),
  );
}

class _Choice extends StatelessWidget {
  const _Choice({
    required this.icon,
    required this.label,
    required this.selected,
    required this.onTap,
  });

  final IconData icon;
  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => KCard(
    onTap: onTap,
    highlight: selected,
    padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
    child: SizedBox(
      height: K.minTap - K.s4,
      child: Row(
        children: [
          Icon(icon, size: 20, color: selected ? K.brand400 : K.muted),
          const SizedBox(width: K.s2),
          Expanded(
            child: Text(
              label,
              maxLines: 1,
              overflow: TextOverflow.ellipsis,
              style: TextStyle(
                fontSize: 14,
                fontWeight: FontWeight.w600,
                color: selected ? K.text : K.textDim,
              ),
            ),
          ),
        ],
      ),
    ),
  );
}
