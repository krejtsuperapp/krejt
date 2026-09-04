import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';

/// Mbështetja: lista e çështjeve të hapura dhe të mbyllura, dhe një bisedë për secilën.
/// Serveri e mbante këtë prej fillimi; aplikacioni thjesht nuk kishte se ku ta tregonte, ndaj një
/// ankesë nuk kishte fare rrugë përveç telefonit.
class SupportScreen extends StatefulWidget {
  const SupportScreen({super.key, required this.api});

  final KrejtApi api;

  @override
  State<SupportScreen> createState() => _SupportScreenState();
}

class _SupportScreenState extends State<SupportScreen> {
  List<SupportTicket> _items = const [];
  ApiError? _error;
  bool _loading = true;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  Future<void> _load() async {
    if (mounted) setState(() => _loading = true);
    try {
      final items = await widget.api.supportTickets();
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

  Future<void> _openTicket(String id) async {
    await Navigator.of(context).push(
      MaterialPageRoute<void>(
        builder: (_) => TicketScreen(api: widget.api, ticketId: id),
      ),
    );
    if (mounted) await _load();
  }

  Future<void> _newTicket() async {
    final created = await Navigator.of(context)
        .push<String>(MaterialPageRoute<String>(builder: (_) => NewTicketScreen(api: widget.api)));
    if (!mounted) return;
    await _load();
    if (created != null && mounted) await _openTicket(created);
  }

  @override
  Widget build(BuildContext context) => Scaffold(
    backgroundColor: K.bg,
    appBar: AppBar(title: Text(context.t('account.support'))),
    body: SafeArea(
      child: Column(
        children: [
          Expanded(
            child: _loading && _items.isEmpty
                ? const Padding(
                    padding: EdgeInsets.all(K.s5),
                    child: KSkeleton(height: 68, count: 3),
                  )
                : _items.isEmpty
                ? Padding(
                    padding: const EdgeInsets.all(K.s5),
                    child: _error != null
                        ? KError(
                            message: context.tError(_error!.messageKey),
                            retryLabel: context.t('common.retry'),
                            onRetry: _load,
                          )
                        : KEmpty(
                            title: context.t('support.empty'),
                            message: context.t('support.empty.hint'),
                            icon: Icons.support_agent_outlined,
                          ),
                  )
                : RefreshIndicator(
                    onRefresh: _load,
                    color: K.brand400,
                    backgroundColor: K.surface2,
                    child: ListView.builder(
                      padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s4),
                      itemCount: _items.length,
                      itemBuilder: (_, i) {
                        final t = _items[i];
                        return Padding(
                          padding: const EdgeInsets.only(bottom: K.s2),
                          child: KCard(
                            onTap: () => _openTicket(t.id),
                            padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
                            child: Row(
                              children: [
                                Expanded(
                                  child: Column(
                                    crossAxisAlignment: CrossAxisAlignment.start,
                                    children: [
                                      Text(
                                        t.subject,
                                        maxLines: 1,
                                        overflow: TextOverflow.ellipsis,
                                        style: const TextStyle(
                                          fontSize: 15,
                                          fontWeight: FontWeight.w600,
                                          color: K.text,
                                        ),
                                      ),
                                      const SizedBox(height: 2),
                                      Text(
                                        '${context.t('support.category.${t.category}')} · ${_when(t.lastMessageAt)}',
                                        style: const TextStyle(fontSize: 12, color: K.muted),
                                      ),
                                    ],
                                  ),
                                ),
                                const SizedBox(width: K.s3),
                                KBadge(
                                  context.t('support.status.${t.status}'),
                                  tone: t.closed ? KTone.neutral : KTone.info,
                                ),
                              ],
                            ),
                          ),
                        );
                      },
                    ),
                  ),
          ),
          Padding(
            padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s5),
            child: KButton(label: context.t('support.new'), onPressed: _newTicket),
          ),
        ],
      ),
    ),
  );

  static String _when(DateTime at) => '${at.day}.${at.month}.${at.year}';
}

/// Një çështje e re. Kategoria dhe përshkrimi mjaftojnë; gjithçka tjetër e di serveri.
class NewTicketScreen extends StatefulWidget {
  const NewTicketScreen({super.key, required this.api});

  final KrejtApi api;

  @override
  State<NewTicketScreen> createState() => _NewTicketScreenState();
}

class _NewTicketScreenState extends State<NewTicketScreen> {
  final _subject = TextEditingController();
  final _body = TextEditingController();
  String _category = supportCategories.first;
  String? _failure;
  bool _busy = false;

  @override
  void dispose() {
    _subject.dispose();
    _body.dispose();
    super.dispose();
  }

  Future<void> _send() async {
    setState(() {
      _busy = true;
      _failure = null;
    });
    try {
      final t = await widget.api.createTicket(
        category: _category,
        subject: _subject.text.trim(),
        body: _body.text.trim(),
      );
      if (mounted) Navigator.of(context).pop(t.id);
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _failure = context.tError(e.messageKey);
        _busy = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    // Të njëjtat kufij si te serveri: një buton i fikur pa shpjegim është më i keq se një
    // gabim i qartë, ndaj as nuk e ndalojmë atë që serveri do ta pranonte.
    final subject = _subject.text.trim();
    final ready = subject.length >= 3 && subject.length <= 120 && _body.text.trim().length >= 3;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('support.new'))),
      body: SafeArea(
        child: ListView(
          padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s8),
          children: [
            Text(
              context.t('support.category'),
              style: const TextStyle(fontSize: 13, color: K.textDim),
            ),
            const SizedBox(height: K.s2),
            Wrap(
              spacing: K.s2,
              runSpacing: K.s2,
              children: [
                for (final c in supportCategories)
                  _Choice(
                    label: context.t('support.category.$c'),
                    selected: _category == c,
                    onTap: () => setState(() => _category = c),
                  ),
              ],
            ),
            const SizedBox(height: K.s5),
            KField(
              controller: _subject,
              label: context.t('support.subject'),
              maxLength: 120,
              onChanged: (_) => setState(() {}),
            ),
            const SizedBox(height: K.s4),
            KField(
              controller: _body,
              label: context.t('support.message'),
              maxLines: 6,
              maxLength: 2000,
              onChanged: (_) => setState(() {}),
            ),
            if (_failure != null) ...[
              const SizedBox(height: K.s4),
              Text(_failure!, style: const TextStyle(fontSize: 13, color: K.danger)),
            ],
            const SizedBox(height: K.s6),
            KButton(
              label: context.t('support.send'),
              busy: _busy,
              onPressed: ready && !_busy ? _send : null,
            ),
          ],
        ),
      ),
    );
  }
}

class _Choice extends StatelessWidget {
  const _Choice({required this.label, required this.selected, required this.onTap});

  final String label;
  final bool selected;
  final VoidCallback onTap;

  @override
  Widget build(BuildContext context) => Semantics(
    button: true,
    selected: selected,
    child: Material(
      color: selected ? K.brand500.withValues(alpha: 0.14) : K.surface,
      borderRadius: BorderRadius.circular(K.rFull),
      child: InkWell(
        borderRadius: BorderRadius.circular(K.rFull),
        onTap: onTap,
        child: Container(
          padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s2),
          decoration: BoxDecoration(
            borderRadius: BorderRadius.circular(K.rFull),
            border: Border.all(color: selected ? K.brand500 : K.line2),
          ),
          child: Text(
            label,
            style: TextStyle(
              fontSize: 13,
              fontWeight: FontWeight.w600,
              color: selected ? K.brand400 : K.textDim,
            ),
          ),
        ),
      ),
    ),
  );
}

/// Biseda e një çështjeje. E mbyllura lexohet, por nuk pranon më mesazhe: më mirë ta thotë hapur
/// se sa ta lërë përdoruesin të shkruajë diçka që nuk e lexon askush.
class TicketScreen extends StatefulWidget {
  const TicketScreen({super.key, required this.api, required this.ticketId});

  final KrejtApi api;
  final String ticketId;

  @override
  State<TicketScreen> createState() => _TicketScreenState();
}

class _TicketScreenState extends State<TicketScreen> {
  final _reply = TextEditingController();
  SupportTicket? _ticket;
  ApiError? _error;
  bool _loading = true;
  bool _sending = false;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
  }

  @override
  void dispose() {
    _reply.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    try {
      final t = await widget.api.supportTicket(widget.ticketId);
      if (!mounted) return;
      setState(() {
        _ticket = t;
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

  Future<void> _send() async {
    final text = _reply.text.trim();
    if (text.isEmpty) return;
    setState(() => _sending = true);
    try {
      await widget.api.replyToTicket(widget.ticketId, text);
      _reply.clear();
      await _load();
    } on ApiError catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
      }
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  Future<void> _close() async {
    final ok = await confirmKSheet(
      context: context,
      title: context.t('support.close.confirm'),
      message: context.t('support.close.body'),
      confirmLabel: context.t('support.close'),
      cancelLabel: context.t('common.no'),
    );
    if (!ok || !mounted) return;
    try {
      await widget.api.closeTicket(widget.ticketId);
      await _load();
    } on ApiError catch (e) {
      if (mounted) {
        ScaffoldMessenger.of(context)
            .showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
      }
    }
  }

  @override
  Widget build(BuildContext context) {
    final t = _ticket;
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(
        title: Text(t?.subject ?? context.t('account.support')),
        actions: [
          if (t != null && !t.closed)
            TextButton(
              onPressed: _close,
              child: Text(
                context.t('support.close'),
                style: const TextStyle(color: K.brand400, fontWeight: FontWeight.w600),
              ),
            ),
        ],
      ),
      body: SafeArea(
        child: _loading
            ? const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 56, count: 4))
            : t == null
            ? Padding(
                padding: const EdgeInsets.all(K.s5),
                child: KError(
                  message: context.tError(_error?.messageKey ?? 'errors.internal'),
                  retryLabel: context.t('common.retry'),
                  onRetry: _load,
                ),
              )
            : Column(
                children: [
                  Expanded(
                    child: ListView(
                      padding: const EdgeInsets.fromLTRB(K.s5, K.s4, K.s5, K.s4),
                      children: [
                        for (final m in t.messages)
                          Align(
                            alignment: m.mine ? Alignment.centerRight : Alignment.centerLeft,
                            child: Container(
                              margin: const EdgeInsets.only(bottom: K.s2),
                              padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
                              constraints: const BoxConstraints(maxWidth: 320),
                              decoration: BoxDecoration(
                                color: m.mine ? K.brand500.withValues(alpha: 0.14) : K.surface,
                                borderRadius: BorderRadius.circular(K.rMd),
                                border: Border.all(color: m.mine ? K.brand500 : K.line),
                              ),
                              child: Text(
                                m.body,
                                style: const TextStyle(fontSize: 14, color: K.text, height: 1.5),
                              ),
                            ),
                          ),
                        if (t.closed)
                          Padding(
                            padding: const EdgeInsets.only(top: K.s4),
                            child: Text(
                              context.t('support.closed.note'),
                              textAlign: TextAlign.center,
                              style: const TextStyle(fontSize: 12, color: K.muted),
                            ),
                          ),
                      ],
                    ),
                  ),
                  if (!t.closed)
                    Padding(
                      padding: const EdgeInsets.fromLTRB(K.s5, 0, K.s5, K.s5),
                      child: Row(
                        children: [
                          Expanded(
                            child: KField(
                              controller: _reply,
                              label: context.t('support.reply'),
                              maxLines: 3,
                            ),
                          ),
                          const SizedBox(width: K.s3),
                          IconButton(
                            onPressed: _sending ? null : _send,
                            icon: const Icon(Icons.send, color: K.brand400),
                          ),
                        ],
                      ),
                    ),
                ],
              ),
      ),
    );
  }
}
