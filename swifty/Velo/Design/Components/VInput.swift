import SwiftUI

struct VInput: View {
    let label: String
    let placeholder: String
    @Binding var text: String
    var maxLength: Int? = nil
    var isSecure: Bool = false
    var keyboardType: UIKeyboardType = .default
    var submitLabel: SubmitLabel = .done
    var onSubmit: (() -> Void)? = nil

    var body: some View {
        VStack(alignment: .leading, spacing: Tokens.Spacing.sm) {
            HStack {
                Text(label)
                    .font(Tokens.Typography.label)
                    .foregroundStyle(Tokens.Colors.foreground)
                Spacer()
                if let maxLength {
                    Text("\(text.count)/\(maxLength)")
                        .font(Tokens.Typography.caption)
                        .foregroundStyle(text.count >= maxLength
                            ? Tokens.Colors.destructive
                            : Tokens.Colors.mutedForeground)
                }
            }

            Group {
                if isSecure {
                    SecureField(placeholder, text: $text)
                } else {
                    TextField(placeholder, text: $text)
                        .keyboardType(keyboardType)
                        .submitLabel(submitLabel)
                        .onSubmit { onSubmit?() }
                }
            }
            .font(Tokens.Typography.bodyBase)
            .padding(.horizontal, Tokens.Spacing.base)
            .frame(height: 48)
            .background(Tokens.Colors.backgroundSubtle)
            .clipShape(RoundedRectangle(cornerRadius: Tokens.Radius.md))
            .overlay(
                RoundedRectangle(cornerRadius: Tokens.Radius.md)
                    .stroke(Tokens.Colors.border, lineWidth: 1)
            )
            .onChange(of: text) { _, newValue in
                if let maxLength, newValue.count > maxLength {
                    text = String(newValue.prefix(maxLength))
                }
            }
        }
    }
}

#Preview {
    @Previewable @State var name = ""
    VInput(label: "Display Name", placeholder: "Enter your name", text: $name, maxLength: 30)
        .padding()
}
