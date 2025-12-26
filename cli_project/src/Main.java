import org.apache.commons.text.WordUtils;

public class Main {
  public static void main(String[] args) {
    String raw = "hello from cli project";
    String friendly = WordUtils.capitalizeFully(raw);
    System.out.println(friendly);
  }
}
