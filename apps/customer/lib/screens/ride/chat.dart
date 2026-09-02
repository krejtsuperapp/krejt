import 'dart:async';

import 'package:flutter/material.dart';
import 'package:krejt_api/krejt_api.dart';
import 'package:krejt_design/krejt_design.dart';
import 'package:krejt_l10n/krejt_l10n.dart';
import 'package:provider/provider.dart';

import '../../state/app_state.dart';

/// Biseda klient–shofer, e hapur nga caktimi deri 24 orë pas përfundimit (§26).
/// Numri i telefonit nuk shfaqet askund: kontakti kalon nga ky kanal.
class ChatScreen extends StatefulWidget {
  const ChatScreen({super.key, required this.rideId});

  final String rideId;

  @override
  State<ChatScreen> createState() => _ChatScreenState();
}

class _ChatScreenState extends State<ChatScreen> {
  static const _pollEvery = Duration(seconds: 5);
  static const _maxBody = 500;

  final _input = TextEditingController();
  final _scroll = ScrollController();

  List<ChatMessage> _messages = const [];
  Timer? _timer;
  bool _sending = false;
  bool _loaded = false;
  String? _error;

  @override
  void initState() {
    super.initState();
    WidgetsBinding.instance.addPostFrameCallback((_) => _load());
    _timer = Timer.periodic(_pollEvery, (_) => _load());
  }

  @override
  void dispose() {
    _timer?.cancel();
    _input.dispose();
    _scroll.dispose();
    super.dispose();
  }

  Future<void> _load() async {
    if (!mounted) return;
    try {
      final items = await context.read<AppState>().api.rideChat(widget.rideId);
      if (!mounted) return;
      final grew = items.length != _messages.length;
      setState(() {
        _messages = items;
        _loaded = true;
        _error = null;
      });
      if (grew) _scrollToEnd();
    } on ApiError catch (e) {
      if (!mounted) return;
      setState(() {
        _loaded = true;
        _error = context.tError(e.messageKey);
      });
    }
  }

  void _scrollToEnd() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (!_scroll.hasClients) return;
      _scroll.animateTo(
        _scroll.position.maxScrollExtent,
        duration: const Duration(milliseconds: 220),
        curve: Curves.easeOut,
      );
    });
  }

  Future<void> _send() async {
    final body = _input.text.trim();
    if (body.isEmpty || _sending) return;
    setState(() => _sending = true);
    final messenger = ScaffoldMessenger.of(context);
    try {
      final sent = await context.read<AppState>().api.sendRideMessage(widget.rideId, body);
      if (!mounted) return;
      _input.clear();
      setState(() => _messages = [..._messages, sent]);
      _scrollToEnd();
    } on ApiError catch (e) {
      if (!mounted) return;
      messenger.showSnackBar(SnackBar(content: Text(context.tError(e.messageKey))));
    } finally {
      if (mounted) setState(() => _sending = false);
    }
  }

  @override
  Widget build(BuildContext context) {
    return Scaffold(
      backgroundColor: K.bg,
      appBar: AppBar(title: Text(context.t('ride.chat.title'))),
      body: SafeArea(
        child: Column(
          children: [
            Expanded(child: _list(context)),
            Padding(
              padding: const EdgeInsets.fromLTRB(K.s4, 0, K.s4, K.s3),
              child: Row(
                crossAxisAlignment: CrossAxisAlignment.end,
                children: [
                  Expanded(
                    child: KField(
                      label: context.t('ride.chat.title'),
                      controller: _input,
                      hint: context.t('ride.chat.placeholder'),
                      maxLength: _maxBody,
                      maxLines: 3,
                      textInputAction: TextInputAction.send,
                      onSubmitted: (_) => _send(),
                    ),
                  ),
                  const SizedBox(width: K.s2),
                  SizedBox(
                    height: K.minTap,
                    child: KButton(
                      label: context.t('ride.chat.send'),
                      expanded: false,
                      busy: _sending,
                      onPressed: _sending ? null : _send,
                    ),
                  ),
                ],
              ),
            ),
          ],
        ),
      ),
    );
  }

  Widget _list(BuildContext context) {
    if (!_loaded) {
      return const Padding(padding: EdgeInsets.all(K.s5), child: KSkeleton(height: 52));
    }
    if (_messages.isEmpty) {
      return Padding(
        padding: const EdgeInsets.all(K.s5),
        child: KEmpty(
          title: context.t('state.empty'),
          message: _error ?? context.t('ride.chat.placeholder'),
          icon: Icons.chat_bubble_outline,
        ),
      );
    }
    return ListView.builder(
      controller: _scroll,
      padding: const EdgeInsets.fromLTRB(K.s4, K.s4, K.s4, K.s2),
      itemCount: _messages.length,
      itemBuilder: (context, i) => _Bubble(message: _messages[i]),
    );
  }
}

class _Bubble extends StatelessWidget {
  const _Bubble({required this.message});

  final ChatMessage message;

  @override
  Widget build(BuildContext context) {
    final mine = message.mine;
    return Align(
      alignment: mine ? Alignment.centerRight : Alignment.centerLeft,
      child: Container(
        constraints: BoxConstraints(maxWidth: MediaQuery.of(context).size.width * 0.78),
        margin: const EdgeInsets.only(bottom: K.s2),
        padding: const EdgeInsets.symmetric(horizontal: K.s4, vertical: K.s3),
        decoration: BoxDecoration(
          color: mine ? K.brand600 : K.surface2,
          borderRadius: BorderRadius.only(
            topLeft: const Radius.circular(K.rMd),
            topRight: const Radius.circular(K.rMd),
            bottomLeft: Radius.circular(mine ? K.rMd : K.rXs),
            bottomRight: Radius.circular(mine ? K.rXs : K.rMd),
          ),
        ),
        child: Column(
          crossAxisAlignment: CrossAxisAlignment.start,
          children: [
            Text(
              message.body,
              style: TextStyle(fontSize: 14, height: 1.4, color: mine ? K.onBrand : K.text),
            ),
            const SizedBox(height: 2),
            Text(
              '${message.createdAt.hour.toString().padLeft(2, '0')}:'
              '${message.createdAt.minute.toString().padLeft(2, '0')}',
              style: TextStyle(fontSize: 11, color: mine ? K.brand100 : K.muted),
            ),
          ],
        ),
      ),
    );
  }
}
