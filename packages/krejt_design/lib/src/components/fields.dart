import 'package:flutter/material.dart';
import 'package:flutter/services.dart';

import '../tokens.dart';

/// Fushë teksti me etiketë, ndihmë dhe gabim inline (§57): tastiera e duhur, autocomplete, validim i qartë.
class KField extends StatelessWidget {
  const KField({
    super.key,
    required this.label,
    this.controller,
    this.hint,
    this.helper,
    this.error,
    this.keyboardType,
    this.textInputAction,
    this.autofillHints,
    this.obscure = false,
    this.enabled = true,
    this.maxLength,
    this.maxLines = 1,
    this.prefix,
    this.suffix,
    this.onChanged,
    this.onSubmitted,
    this.inputFormatters,
    this.autofocus = false,
  });

  final String label;
  final TextEditingController? controller;
  final String? hint;
  final String? helper;
  final String? error;
  final TextInputType? keyboardType;
  final TextInputAction? textInputAction;
  final Iterable<String>? autofillHints;
  final bool obscure;
  final bool enabled;
  final int? maxLength;
  final int maxLines;
  final Widget? prefix;
  final Widget? suffix;
  final ValueChanged<String>? onChanged;
  final ValueChanged<String>? onSubmitted;
  final List<TextInputFormatter>? inputFormatters;
  final bool autofocus;

  @override
  Widget build(BuildContext context) => Column(
    crossAxisAlignment: CrossAxisAlignment.start,
    children: [
      Text(
        label,
        style: const TextStyle(color: K.textDim, fontSize: 13.5, fontWeight: FontWeight.w600),
      ),
      const SizedBox(height: 6),
      TextField(
        controller: controller,
        enabled: enabled,
        obscureText: obscure,
        keyboardType: keyboardType,
        textInputAction: textInputAction,
        autofillHints: autofillHints,
        maxLength: maxLength,
        maxLines: maxLines,
        autofocus: autofocus,
        inputFormatters: inputFormatters,
        onChanged: onChanged,
        onSubmitted: onSubmitted,
        style: const TextStyle(color: K.text, fontSize: 16),
        decoration: InputDecoration(
          hintText: hint,
          prefixIcon: prefix,
          suffixIcon: suffix,
          counterText: '',
          errorText: error,
          helperText: helper,
          helperStyle: const TextStyle(color: K.muted, fontSize: 12.5),
        ),
      ),
    ],
  );
}

/// Fushë e kodit OTP: 6 shifra, ngjitje e shpejtë, dërgim automatik kur plotësohet (§57, §53).
class KOtpField extends StatefulWidget {
  const KOtpField({
    super.key,
    required this.onCompleted,
    this.length = 6,
    this.error,
    this.enabled = true,
  });

  final ValueChanged<String> onCompleted;
  final int length;
  final String? error;
  final bool enabled;

  @override
  State<KOtpField> createState() => _KOtpFieldState();
}

class _KOtpFieldState extends State<KOtpField> {
  final _controller = TextEditingController();
  final _focus = FocusNode();

  @override
  void initState() {
    super.initState();
    _controller.addListener(() {
      setState(() {});
      if (_controller.text.length == widget.length) widget.onCompleted(_controller.text);
    });
    WidgetsBinding.instance.addPostFrameCallback((_) => _focus.requestFocus());
  }

  @override
  void dispose() {
    _controller.dispose();
    _focus.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final code = _controller.text;
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Stack(
          children: [
            Opacity(
              opacity: 0,
              child: TextField(
                controller: _controller,
                focusNode: _focus,
                enabled: widget.enabled,
                keyboardType: TextInputType.number,
                autofillHints: const [AutofillHints.oneTimeCode],
                maxLength: widget.length,
                inputFormatters: [FilteringTextInputFormatter.digitsOnly],
              ),
            ),
            GestureDetector(
              onTap: () => _focus.requestFocus(),
              child: Row(
                mainAxisAlignment: MainAxisAlignment.spaceBetween,
                children: List.generate(widget.length, (i) {
                  final filled = i < code.length;
                  final active = i == code.length && _focus.hasFocus;
                  return Container(
                    width: 46,
                    height: 56,
                    alignment: Alignment.center,
                    decoration: BoxDecoration(
                      color: K.surface2,
                      borderRadius: BorderRadius.circular(K.rMd),
                      border: Border.all(
                        color: widget.error != null ? K.danger : (active ? K.brand500 : K.line),
                        width: active ? 1.6 : 1,
                      ),
                    ),
                    child: Text(
                      filled ? code[i] : '',
                      style: const TextStyle(
                        fontSize: 22,
                        fontWeight: FontWeight.w700,
                        color: K.text,
                      ),
                    ),
                  );
                }),
              ),
            ),
          ],
        ),
        if (widget.error != null) ...[
          const SizedBox(height: K.s2),
          Text(widget.error!, style: const TextStyle(color: K.danger, fontSize: 13)),
        ],
      ],
    );
  }
}
