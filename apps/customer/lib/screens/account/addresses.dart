import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

const _labels = ['home', 'work', 'other'];

String addressLabelKey(String label) =>
    'account.address.label.${_labels.contains(label) ? label : 'other'}';

IconData addressIcon(String label) {
  switch (label) {
    case 'home':
      return Icons.home_outlined;
    case 'work':
      return Icons.work_outline;
    default:
      return Icons.place_outlined;
  }
}

/// Adresat e ruajtura. Vetëm Kosova mbulohet, ndaj koordinatat vijnë nga zgjedhja në hartë
/// dhe jo nga shkrimi i lirë — deri sa harta të hyjë, adresa ruhet me qytetin dhe rrugën (§17).
class AddressesScreen extends StatefulWidget {
  const AddressesScreen({super.key});

  @override
  State<AddressesScreen> createState() => _AddressesScreenState();
}

class _AddressesScreenState extends State<AddressesScreen> {
  List<Address> _items = const [];
  bool _loading = true;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final items = await context.read<AppState>().api.addresses();
      if (!mounted) return;
      setState(() {
        _items = items;
        _error = null;
        _loading = false;
      });
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = e;
        _loading = false;
      });
    }
  }

  Future<void> _add() async {
    final created = await Navigator.of(context)
        .push<bool>(MaterialPageRoute(builder: (_) => const AddAddressScreen()));
    if (created == true) await _load();
  }

  Future<void> _delete(Address a) async {
    final ok = await confirmKSheet(
      context: context,
      title: context.t('account.address.delete.confirm'),
      message: a.line1,
      confirmLabel: context.t('common.delete'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    final messenger = ScaffoldMessenger.of(context);
    try {
      await context.read<AppState>().api.deleteAddress(a.id);
      await _load();
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.addresses'))),
      floatingActionButton: FloatingActionButton.extended(
        onPressed: _add,
        backgroundColor: K.brand500,
        foregroundColor: K.onBrand,
        icon: const Icon(Icons.add),
        label: Text(context.t('account.address.add')),
      ),
      body: SafeArea(child: _body(context)),
    );
  }

  Widget _body(BuildContext context) {
    if (_loading) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 72));
    }
    if (_error != null) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KError(
          message: context.tError(_error!.messageKey),
          retryLabel: context.t('common.retry'),
          onRetry: _load,
        ),
      );
    }
    if (_items.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KEmpty(
          title: context.t('account.address.empty'),
          message: context.t('account.address.empty.hint'),
          icon: Icons.place_outlined,
          action: context.t('account.address.add'),
          onAction: _add,
        ),
      );
    }
    return RefreshIndicator(
      onRefresh: _load,
      color: K.brand400,
      backgroundColor: K.surface2,
      child: ListView.builder(
        padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, 96),
        itemCount: _items.length,
        itemBuilder: (context, i) {
          final a = _items[i];
          return Padding(
            padding: const EdgeInsets.only(bottom: K.s2),
            child: KCard(
              child: Row(
                children: [
                  Icon(addressIcon(a.label), size: 20, color: K.brand400),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          a.name ?? context.t(addressLabelKey(a.label)),
                          style: const TextStyle(
                            fontSize: 15,
                            fontWeight: FontWeight.w600,
                            color: K.text,
                          ),
                        ),
                        const SizedBox(height: 2),
                        Text(
                          '${a.line1}, ${a.city}',
                          maxLines: 1,
                          overflow: TextOverflow.ellipsis,
                          style: const TextStyle(fontSize: 13, color: K.muted),
                        ),
                      ],
                    ),
                  ),
                  IconButton(
                    icon: const Icon(Icons.delete_outline, color: K.muted),
                    tooltip: context.t('account.address.delete'),
                    onPressed: () => _delete(a),
                  ),
                ],
              ),
            ),
          );
        },
      ),
    );
  }
}

class AddAddressScreen extends StatefulWidget {
  const AddAddressScreen({super.key});

  @override
  State<AddAddressScreen> createState() => _AddAddressScreenState();
}

class _AddAddressScreenState extends State<AddAddressScreen> {
  final _line1 = TextEditingController();
  final _city = TextEditingController(text: 'Prishtinë');
  final _instructions = TextEditingController();

  String _label = 'home';
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _line1.dispose();
    _city.dispose();
    _instructions.dispose();
    super.dispose();
  }

  Future<void> _save() async {
    if (_line1.text.trim().isEmpty || _city.text.trim().isEmpty) {
      setState(() => _error = context.t('errors.validation'));
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      // Koordinatat vijnë nga zgjedhja në hartë në fazën e udhëtimit; deri atëherë
      // serveri i pranon si qendër e qytetit dhe i saktëson kur klienti zgjedh pikën.
      await context.read<AppState>().api.addAddress(
        Address(
          id: '',
          label: _label,
          line1: _line1.text.trim(),
          city: _city.text.trim(),
          lat: 42.6629,
          lng: 21.1655,
          instructions: _instructions.text.trim().isEmpty ? null : _instructions.text.trim(),
        ),
      );
      if (!mounted) return;
      Navigator.of(context).pop(true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = context.tError(e.messageKey));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('account.address.add'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.all(K.s5),
          children: [
            Wrap(
              spacing: K.s2,
              children: [
                for (final l in _labels)
                  ChoiceChip(
                    label: Text(context.t(addressLabelKey(l))),
                    selected: _label == l,
                    onSelected: (_) => setState(() => _label = l),
                  ),
              ],
            ),
            const SizedBox(height: K.s4),
            KField(
              label: context.t('account.address.line1'),
              controller: _line1,
              textInputAction: TextInputAction.next,
              autofocus: true,
            ),
            const SizedBox(height: K.s4),
            KField(label: context.t('account.address.city'), controller: _city),
            const SizedBox(height: K.s4),
            KField(
              label: context.t('account.address.instructions'),
              controller: _instructions,
              maxLength: 200,
              maxLines: 3,
            ),
            if (_error != null) ...[
              const SizedBox(height: K.s3),
              Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
            ],
            const SizedBox(height: K.s6),
            KButton(label: context.t('common.save'), busy: _busy, onPressed: _busy ? null : _save),
          ],
        ),
      ),
    );
  }
}
