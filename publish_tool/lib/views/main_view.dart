import 'package:fluent_ui/fluent_ui.dart';
import 'package:flutter/widgets.dart';

class MainView extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return Row(
      children: [
        Container(width: 200, child: ListView()),
        Expanded(
          child: Container(child: Text("右边"), color: Colors.orange),
        ),
      ],
    );
  }
}
