import 'dart:async';

import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../state/app_state.dart';

const serviceCategoryIds = [
  'electrician',
  'plumber',
  'cleaning',
  'ac',
  'appliance',
  'moving',
  'handyman',
];

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

String serviceStateKey(ServiceState s) {
  switch (s) {
    case ServiceState.open:
      return 'service.state.open';
    case ServiceState.booked:
      return 'service.state.booked';
    case ServiceState.inProgress:
      return 'service.state.in_progress';
    case ServiceState.completed:
      return 'service.state.completed';
    case ServiceState.cancelled:
      return 'service.state.cancelled';
    case ServiceState.noOffers:
      return 'service.state.no_offers';
  }
}

/// Ekrani i mjeshtrit: punët e hapura në kategoritë e tij, ofertat e veta dhe puna në ecje.
/// Pa profil shfaqet aplikimi; pa miratim, arsyeja — kurrë një listë bosh pa shpjegim (§22).
class ServicesScreen extends StatefulWidget {
  const ServicesScreen({super.key});

  @override
  State<ServicesScreen> createState() => _ServicesScreenState();
}

class _ServicesScreenState extends State<ServicesScreen> {
  ServiceProviderProfile? _profile;
  List<ServiceOpenRequest> _open = const [];
  List<ServiceRequest> _jobs = const [];
  bool _loading = true;
  bool _noProfile = false;
  ApiError? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    final api = context.read<AppState>().api;
    try {
      final profile = await api.serviceProviderProfile();
      if (!mounted) return;
      setState(() {
        _profile = profile;
        _noProfile = false;
      });
      if (profile.approved) {
        final results = await Future.wait([api.openServiceRequests(), api.myServiceJobs()]);
        if (!mounted) return;
        setState(() {
          _open = results[0] as List<ServiceOpenRequest>;
          _jobs = results[1] as List<ServiceRequest>;
        });
      }
      if (mounted) setState(() => _error = null);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        if (e.isNotFound) {
          _noProfile = true;
          _profile = null;
        } else {
          _error = e;
        }
      });
    } finally {
      if (mounted) setState(() => _loading = false);
    }
  }

  ServiceRequest? get _activeJob {
    for (final j in _jobs) {
      if (j.isActive) return j;
    }
    return null;
  }

  Future<void> _offer(ServiceOpenRequest request) async {
    final price = TextEditingController(
      text: request.myOffer == null ? '' : (request.myOffer!.priceMinor / 100).toStringAsFixed(0),
    );
    final note = TextEditingController(text: request.myOffer?.note ?? '');
    final sent = await showKSheet<bool>(
      context: context,
      title: context.t('provider.offer.title'),
      subtitle: request.title,
      scrollable: true,
      child: _OfferForm(requestId: request.id, price: price, note: note),
    );
    price.dispose();
    note.dispose();
    if (sent != true || !mounted) return;
    await _load();
  }

  Future<void> _step(Future<ServiceRequest> Function() run) async {
    final messenger = ScaffoldMessenger.of(context);
    String? failure;
    try {
      await run();
    } on ApiError catch (e) {
      // Teksti lexohet para pritjes; pas saj konteksti mund të mos jetë më i njëjti.
      if (mounted) failure = context.tError(e.messageKey);
    }
    if (failure != null) messenger.showSnackBar(SnackBar(content: Text(failure)));
    if (mounted) await _load();
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    if (_loading && _profile == null && !_noProfile) {
      return const SafeArea(
        child: Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 88, count: 3)),
      );
    }
    if (_noProfile) return SafeArea(child: _ApplyCard(onApplied: _load));
    final profile = _profile;
    if (profile == null) {
      return SafeArea(
        child: Padding(
          padding: const EdgeInsets.all(K.s5),
          child: KError(
            message: context.tError(_error?.messageKey ?? 'errors.internal'),
            retryLabel: context.t('common.retry'),
            onRetry: _load,
          ),
        ),
      );
    }

    final active = _activeJob;
    return SafeArea(
      child: RefreshIndicator(
        onRefresh: _load,
        color: K.brand400,
        backgroundColor: K.surface2,
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Row(
              children: [
                Expanded(
                  child: Text(
                    context.t('provider.title'),
                    style: const TextStyle(
                      fontSize: 22,
                      fontWeight: FontWeight.w800,
                      letterSpacing: -0.4,
                      color: K.text,
                    ),
                  ),
                ),
                if (profile.approved)
                  KPill(
                    context.t('service.provider.jobs', {'n': '${profile.jobsDone}'}),
                    icon: Icons.verified_outlined,
                    neon: true,
                  )
                else
                  KPill(context.t('driver.pending'), icon: Icons.hourglass_bottom),
              ],
            ),
            if (!profile.approved) ...[
              const SizedBox(height: K.s4),
              KCard(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      context.t(
                        profile.status == 'suspended' ? 'provider.suspended' : 'driver.pending',
                      ),
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                    const SizedBox(height: K.s2),
                    Text(
                      profile.suspendedReason ?? context.t('provider.pending.hint'),
                      style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
                    ),
                  ],
                ),
              ),
            ],
            if (active != null) ...[
              const SizedBox(height: K.s4),
              _ActiveJobCard(
                job: active,
                locale: locale,
                onStart: () => _step(() => context.read<AppState>().api.startServiceJob(active.id)),
                onComplete: () =>
                    _step(() => context.read<AppState>().api.completeServiceJob(active.id)),
                onRelease: () async {
                  final ok = await confirmKSheet(
                    context: context,
                    title: context.t('provider.release.confirm'),
                    message: context.t('provider.release.body'),
                    confirmLabel: context.t('courier.parcel.release'),
                    cancelLabel: context.t('common.no'),
                    destructive: true,
                  );
                  if (!ok || !mounted) return;
                  await _step(() => context.read<AppState>().api.releaseServiceJob(active.id));
                },
              ),
            ],
            if (profile.approved) ...[
              const SizedBox(height: K.s5),
              KSectionHeader(context.t('provider.open')),
              const SizedBox(height: K.s3),
              if (_open.isEmpty)
                KEmpty(
                  title: context.t('provider.open.empty'),
                  message: context.t('provider.open.empty.hint'),
                  icon: Icons.inbox_outlined,
                )
              else
                for (final r in _open)
                  Padding(
                    padding: const EdgeInsets.only(bottom: K.s2),
                    child: _OpenRequestCard(
                      request: r,
                      locale: locale,
                      busy: active != null,
                      onOffer: () => _offer(r),
                    ),
                  ),
            ],
          ],
        ),
      ),
    );
  }
}

/// Aplikimi si mjeshtër: kategoritë dhe qyteti. Miratimin e jep Operacionet.
class _ApplyCard extends StatefulWidget {
  const _ApplyCard({required this.onApplied});

  final Future<void> Function() onApplied;

  @override
  State<_ApplyCard> createState() => _ApplyCardState();
}

class _ApplyCardState extends State<_ApplyCard> {
  final _city = TextEditingController();
  final _business = TextEditingController();
  final _selected = <String>{'handyman'};
  bool _busy = false;
  String? _error;

  @override
  void dispose() {
    _city.dispose();
    _business.dispose();
    super.dispose();
  }

  Future<void> _submit() async {
    if (_city.text.trim().isEmpty || _selected.isEmpty) {
      setState(() => _error = context.t('errors.validation'));
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    try {
      await context.read<AppState>().api.applyAsServiceProvider(
        categories: _selected.toList(),
        city: _city.text.trim(),
        businessName: _business.text.trim().isEmpty ? null : _business.text.trim(),
      );
      await widget.onApplied();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) => ListView(
    padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
    children: [
      Text(
        context.t('provider.apply.title'),
        style: const TextStyle(fontSize: 22, fontWeight: FontWeight.w800, color: K.text),
      ),
      const SizedBox(height: K.s2),
      Text(
        context.t('provider.apply.intro'),
        style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
      ),
      const SizedBox(height: K.s5),
      KSectionHeader(context.t('service.category')),
      const SizedBox(height: K.s3),
      Wrap(
        spacing: K.s2,
        runSpacing: K.s2,
        children: [
          for (final c in serviceCategoryIds)
            FilterChip(
              avatar: Icon(serviceIconFor(c), size: 18),
              label: Text(context.t('service.category.$c')),
              selected: _selected.contains(c),
              onSelected: (on) => setState(() {
                if (on) {
                  _selected.add(c);
                } else if (_selected.length > 1) {
                  _selected.remove(c);
                }
                _error = null;
              }),
            ),
        ],
      ),
      const SizedBox(height: K.s5),
      KField(
        label: context.t('provider.apply.city'),
        controller: _city,
        hint: 'Prishtinë',
        textInputAction: TextInputAction.next,
        onChanged: (_) => setState(() => _error = null),
      ),
      const SizedBox(height: K.s3),
      KField(
        label: context.t('provider.apply.business'),
        controller: _business,
        hint: context.t('provider.apply.business.hint'),
        textInputAction: TextInputAction.done,
        onSubmitted: (_) => _submit(),
      ),
      if (_error != null) ...[
        const SizedBox(height: K.s4),
        Text(_error!, style: const TextStyle(fontSize: 13, color: K.danger)),
      ],
      const SizedBox(height: K.s6),
      KButton(
        label: context.t('provider.apply.submit'),
        icon: Icons.arrow_forward,
        busy: _busy,
        onPressed: _busy ? null : _submit,
      ),
    ],
  );
}

class _OfferForm extends StatefulWidget {
  const _OfferForm({required this.requestId, required this.price, required this.note});

  final String requestId;
  final TextEditingController price;
  final TextEditingController note;

  @override
  State<_OfferForm> createState() => _OfferFormState();
}

class _OfferFormState extends State<_OfferForm> {
  bool _busy = false;
  String? _error;

  Future<void> _send() async {
    final euros = double.tryParse(widget.price.text.replaceAll(',', '.').trim());
    if (euros == null || euros <= 0) {
      setState(() => _error = context.t('errors.validation'));
      return;
    }
    setState(() {
      _busy = true;
      _error = null;
    });
    final navigator = Navigator.of(context);
    try {
      await context.read<AppState>().api.makeServiceOffer(
        widget.requestId,
        priceMinor: (euros * 100).round(),
        note: widget.note.text.trim().isEmpty ? null : widget.note.text.trim(),
      );
      navigator.pop(true);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _error = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.stretch,
    children: [
      KField(
        label: context.t('provider.offer.price'),
        controller: widget.price,
        hint: '25',
        error: _error,
        keyboardType: const TextInputType.numberWithOptions(decimal: true),
        inputFormatters: [FilteringTextInputFormatter.allow(RegExp(r'[0-9.,]'))],
        autofocus: true,
        onChanged: (_) => setState(() => _error = null),
      ),
      const SizedBox(height: K.s3),
      KField(
        label: context.t('provider.offer.note'),
        controller: widget.note,
        hint: context.t('provider.offer.note.hint'),
        maxLines: 3,
        maxLength: 300,
      ),
      const SizedBox(height: K.s5),
      KButton(
        label: context.t('provider.offer.send'),
        busy: _busy,
        onPressed: _busy ? null : _send,
      ),
    ],
  );
}

class _OpenRequestCard extends StatelessWidget {
  const _OpenRequestCard({
    required this.request,
    required this.locale,
    required this.busy,
    required this.onOffer,
  });

  final ServiceOpenRequest request;
  final String locale;
  final bool busy;
  final VoidCallback onOffer;

  @override
  Widget build(BuildContext context) => KCard(
    highlight: request.myOffer != null,
    child: Column(
      crossAxisAlignment: CrossAxisAlignment.stretch,
      children: [
        Row(
          children: [
            Icon(serviceIconFor(request.categoryId), size: 20, color: K.brand400),
            const SizedBox(width: K.s2),
            Expanded(
              child: Text(
                request.title,
                maxLines: 1,
                overflow: TextOverflow.ellipsis,
                style: const TextStyle(fontSize: 16, fontWeight: FontWeight.w700, color: K.text),
              ),
            ),
            KChip(request.city),
          ],
        ),
        const SizedBox(height: K.s2),
        Text(
          request.description,
          maxLines: 3,
          overflow: TextOverflow.ellipsis,
          style: const TextStyle(fontSize: 13, color: K.textDim, height: 1.4),
        ),
        if (request.myOffer != null) ...[
          const SizedBox(height: K.s3),
          Row(
            children: [
              const Icon(Icons.check_circle, size: 16, color: K.brand400),
              const SizedBox(width: K.s2),
              Text(
                context.t('provider.offer.sent'),
                style: const TextStyle(fontSize: 12, color: K.muted),
              ),
              const Spacer(),
              KMoney(
                request.myOffer!.priceMinor,
                currency: request.myOffer!.currency,
                locale: locale,
                size: 16,
              ),
            ],
          ),
        ],
        const SizedBox(height: K.s4),
        KOutlineButton(
          label: context.t(request.myOffer == null ? 'provider.offer.send' : 'provider.offer.edit'),
          icon: Icons.local_offer_outlined,
          onPressed: busy ? null : onOffer,
        ),
      ],
    ),
  );
}

class _ActiveJobCard extends StatelessWidget {
  const _ActiveJobCard({
    required this.job,
    required this.locale,
    required this.onStart,
    required this.onComplete,
    required this.onRelease,
  });

  final ServiceRequest job;
  final String locale;
  final VoidCallback onStart;
  final VoidCallback onComplete;
  final VoidCallback onRelease;

  @override
  Widget build(BuildContext context) {
    final booked = job.state == ServiceState.booked;
    return KCard(
      highlight: true,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Text(
                  job.title,
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 17, fontWeight: FontWeight.w700, color: K.text),
                ),
              ),
              if (job.priceMinor != null)
                KMoney(job.priceMinor!, currency: job.currency, locale: locale, size: 20),
            ],
          ),
          const SizedBox(height: K.s2),
          Text(
            context.t(serviceStateKey(job.state)),
            style: const TextStyle(fontSize: 13, color: K.muted),
          ),
          const SizedBox(height: K.s3),
          KRow(context.t('service.form.address'), job.addressLine1),
          KRow(
            context.t('ride.payment'),
            context.t(job.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash'),
          ),
          const SizedBox(height: K.s4),
          KButton(
            label: context.t(booked ? 'provider.start' : 'provider.complete'),
            icon: booked ? Icons.play_arrow : Icons.check,
            onPressed: booked ? onStart : onComplete,
          ),
          if (booked) ...[
            const SizedBox(height: K.s2),
            KOutlineButton(
              label: context.t('courier.parcel.release'),
              icon: Icons.undo,
              danger: true,
              onPressed: onRelease,
            ),
          ],
        ],
      ),
    );
  }
}
