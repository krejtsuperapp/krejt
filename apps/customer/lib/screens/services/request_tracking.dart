import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import 'package:krejt_screens/krejt_screens.dart';

import '../../state/app_state.dart';
import 'new_request.dart';

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

/// Kërkesa për mjeshtër: ofertat vijnë me çmimin e secilit dhe klienti zgjedh. Pasi zgjedh,
/// ekrani tregon mjeshtrin dhe rrjedhën e punës (§22).
class ServiceTrackingScreen extends StatefulWidget {
  const ServiceTrackingScreen({super.key, required this.requestId});

  final String requestId;

  @override
  State<ServiceTrackingScreen> createState() => _ServiceTrackingScreenState();
}

class _ServiceTrackingScreenState extends State<ServiceTrackingScreen> {
  static const _pollEvery = Duration(seconds: 10);

  Timer? _timer;
  ServiceRequest? _request;
  ApiError? _error;
  bool _busy = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _poll());
    _timer = Timer.periodic(_pollEvery, (_) => _poll());
  }

  @override
  void dispose() {
    _timer?.cancel();
    super.dispose();
  }

  Future<void> _poll() async {
    if (!mounted) return;
    try {
      final r = await context.read<AppState>().api.serviceRequest(widget.requestId);
      if (!mounted) return;
      setState(() {
        _request = r;
        _error = null;
      });
      if (r.isFinished) {
        _timer?.cancel();
        unawaited(context.read<AppState>().refreshHome());
      }
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() => _error = e);
    }
  }

  Future<void> _accept(ServiceOffer offer) async {
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final r = await context.read<AppState>().api.acceptServiceOffer(widget.requestId, offer.id);
      if (!mounted) return;
      setState(() => _request = r);
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  Future<void> _cancel() async {
    final ok = await confirmKSheet(
      context: context,
      title: context.t('service.cancel.confirm'),
      message: context.t('service.cancel.body'),
      confirmLabel: context.t('service.cancel'),
      cancelLabel: context.t('common.no'),
      destructive: true,
    );
    if (!ok || !mounted) return;
    setState(() => _busy = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final r = await context.read<AppState>().api.cancelServiceRequest(widget.requestId);
      if (!mounted) return;
      setState(() => _request = r);
      _timer?.cancel();
      unawaited(context.read<AppState>().refreshHome());
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _busy = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    final locale = context.watch<AppState>().locale;
    final r = _request;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('service.title'))),
      body: SafeArea(
        child: r == null
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: _error == null
                    ? KLoading(label: context.t('common.loading'))
                    : KError(
                        message: context.tError(_error!.messageKey),
                        retryLabel: context.t('common.retry'),
                        onRetry: _poll,
                      ),
              )
            : RefreshIndicator(
                onRefresh: _poll,
                color: K.brand400,
                backgroundColor: K.surface2,
                child: _content(context, r, locale),
              ),
      ),
    );
  }

  Widget _content(BuildContext context, ServiceRequest r, String locale) => ListView(
    padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
    children: [
      Row(
        children: [
          Container(
            width: 44,
            height: 44,
            alignment: Alignment.center,
            decoration: BoxDecoration(
              color: K.surface2,
              borderRadius: BorderRadius.circular(K.rSm),
            ),
            child: Icon(serviceIconFor(r.categoryId), size: 22, color: K.brand400),
          ),
          const SizedBox(width: K.s3),
          Expanded(
            child: Column(
              crossAxisAlignment: CrossAxisAlignment.start,
              children: [
                Text(
                  context.t(serviceStateKey(r.state)),
                  style: const TextStyle(fontSize: 20, fontWeight: FontWeight.w700, color: K.text),
                ),
                Text(
                  '${r.title} · ${r.code}',
                  maxLines: 1,
                  overflow: TextOverflow.ellipsis,
                  style: const TextStyle(fontSize: 13, color: K.muted),
                ),
              ],
            ),
          ),
        ],
      ),
      const SizedBox(height: K.s4),
      KCard(
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              r.description,
              style: const TextStyle(fontSize: 14, color: K.textDim, height: 1.45),
            ),
            const SizedBox(height: K.s3),
            KRow(context.t('service.form.address'), r.addressLine1),
            if (r.preferredAt != null) KRow(context.t('service.form.when'), _when(r.preferredAt!)),
            KRow(
              context.t('ride.payment'),
              context.t(r.paymentMethod == 'wallet' ? 'ride.payment.wallet' : 'ride.payment.cash'),
            ),
            if (r.priceMinor != null) ...[
              const KMoneyDivider(),
              KMoneyRow(
                context.t('cart.total'),
                r.priceMinor!,
                currency: r.currency,
                locale: locale,
                total: true,
              ),
            ],
          ],
        ),
      ),
      if (r.state == ServiceState.open) ...[
        const SizedBox(height: K.s5),
        KSectionHeader(context.t('service.offers')),
        const SizedBox(height: K.s3),
        if (r.offers.isEmpty)
          KEmpty(
            title: context.t('service.offers.empty'),
            message: context.t('service.offers.empty.hint'),
            icon: Icons.hourglass_bottom,
          )
        else
          for (final o in r.offers)
            Padding(
              padding: const EdgeInsets.only(bottom: K.s2),
              child: _OfferCard(offer: o, locale: locale, busy: _busy, onAccept: () => _accept(o)),
            ),
      ],
      if (r.provider != null && r.state != ServiceState.open) ...[
        const SizedBox(height: K.s5),
        KCard(
          child: Row(
            children: [
              Container(
                width: 46,
                height: 46,
                alignment: Alignment.center,
                decoration: BoxDecoration(
                  color: K.surface3,
                  borderRadius: BorderRadius.circular(K.rFull),
                ),
                child: const Icon(Icons.handyman_outlined, color: K.textDim),
              ),
              const SizedBox(width: K.s3),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      r.provider!.displayName,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                    Text(
                      [
                        r.provider!.city,
                        context.t('service.provider.jobs', {'n': '${r.provider!.jobsDone}'}),
                      ].join(' · '),
                      style: const TextStyle(fontSize: 13, color: K.muted),
                    ),
                  ],
                ),
              ),
              if (r.provider!.rating != null)
                KBadge('★ ${r.provider!.rating!.toStringAsFixed(1)}', tone: KTone.brand),
            ],
          ),
        ),
      ],
      const SizedBox(height: K.s5),
      _Timeline(request: r),
      const SizedBox(height: K.s5),
      KOutlineButton(
        label: context.t('account.support'),
        icon: Icons.support_agent_outlined,
        onPressed: () => Navigator.of(context).push(
          MaterialPageRoute<void>(
            builder: (_) => SupportScreen(
              api: context.read<AppState>().api,
              about: const TicketSubject(category: 'other').copyWith(requestId: r.id),
            ),
          ),
        ),
      ),
      const SizedBox(height: K.s3),
      if (r.canCancel)
        KOutlineButton(
          label: context.t('service.cancel'),
          icon: Icons.close,
          danger: true,
          onPressed: _busy ? null : _cancel,
        ),
      if (r.isFinished)
        KButton(label: context.t('common.close'), onPressed: () => Navigator.of(context).pop()),
    ],
  );

  static String _when(DateTime at) =>
      '${at.day}.${at.month} · ${at.hour.toString().padLeft(2, '0')}:${at.minute.toString().padLeft(2, '0')}';
}

class _OfferCard extends StatelessWidget {
  const _OfferCard({
    required this.offer,
    required this.locale,
    required this.busy,
    required this.onAccept,
  });

  final ServiceOffer offer;
  final String locale;
  final bool busy;
  final VoidCallback onAccept;

  @override
  Widget build(BuildContext context) {
    final p = offer.provider;
    return KCard(
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          Row(
            children: [
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      p?.displayName ?? '—',
                      maxLines: 1,
                      overflow: TextOverflow.ellipsis,
                      style: const TextStyle(
                        fontSize: 16,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                    const SizedBox(height: 2),
                    Text(
                      [
                        if (p != null) p.city,
                        if (p != null) context.t('service.provider.jobs', {'n': '${p.jobsDone}'}),
                        if (p?.rating != null) '★ ${p!.rating!.toStringAsFixed(1)}',
                      ].join(' · '),
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                  ],
                ),
              ),
              KMoney(offer.priceMinor, currency: offer.currency, locale: locale, size: 20),
            ],
          ),
          if (offer.note != null && offer.note!.isNotEmpty) ...[
            const SizedBox(height: K.s3),
            Text(offer.note!, style: const TextStyle(fontSize: 13, color: K.textDim, height: 1.4)),
          ],
          if (offer.canStartAt != null) ...[
            const SizedBox(height: K.s2),
            Text(
              context.t('service.offer.can_start', {
                'when':
                    '${offer.canStartAt!.day}.${offer.canStartAt!.month} '
                    '${offer.canStartAt!.hour.toString().padLeft(2, '0')}:'
                    '${offer.canStartAt!.minute.toString().padLeft(2, '0')}',
              }),
              style: const TextStyle(fontSize: 12, color: K.muted),
            ),
          ],
          const SizedBox(height: K.s4),
          KButton(
            label: context.t('service.offer.choose'),
            icon: Icons.check,
            busy: busy,
            onPressed: busy ? null : onAccept,
          ),
        ],
      ),
    );
  }
}

class _Timeline extends StatelessWidget {
  const _Timeline({required this.request});

  final ServiceRequest request;

  @override
  Widget build(BuildContext context) {
    final steps = <MapEntry<String, DateTime?>>[
      MapEntry('service.state.open', request.createdAt),
      MapEntry('service.state.booked', request.bookedAt),
      MapEntry('service.state.in_progress', request.startedAt),
      MapEntry('service.state.completed', request.completedAt),
    ];
    return KCard(
      child: Column(
        children: [
          for (final step in steps)
            SizedBox(
              height: 34,
              child: Row(
                children: [
                  Icon(
                    step.value == null ? Icons.circle_outlined : Icons.check_circle,
                    size: 18,
                    color: step.value == null ? K.line2 : K.ok,
                  ),
                  const SizedBox(width: K.s3),
                  Expanded(
                    child: Text(
                      context.t(step.key),
                      style: TextStyle(
                        fontSize: 14,
                        color: step.value == null ? K.muted : K.textDim,
                      ),
                    ),
                  ),
                  if (step.value != null)
                    Text(
                      '${step.value!.hour.toString().padLeft(2, '0')}:'
                      '${step.value!.minute.toString().padLeft(2, '0')}',
                      style: const TextStyle(fontSize: 12, color: K.muted),
                    ),
                ],
              ),
            ),
        ],
      ),
    );
  }
}
